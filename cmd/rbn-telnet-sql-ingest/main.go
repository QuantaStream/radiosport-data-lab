package main

import (
	"bufio"
	"context"
	"database/sql"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/QuantaStream/radiosport-data-lab/internal/callsign"
	"github.com/QuantaStream/radiosport-data-lab/internal/qrz"
	"github.com/QuantaStream/radiosport-data-lab/internal/rbn"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	rbnAddr := flag.String("rbn-addr", "telnet.reversebeacon.net:7000", "RBN telnet address")
	loginCall := flag.String("login-call", "N7ZG", "callsign sent to the RBN telnet login prompt")
	dsn := flag.String("mysql-dsn", "qstream@tcp(127.0.0.1:4000)/quanta", "MySQL-compatible DSN for QuantaStream")
	batchSize := flag.Int("batch-size", rbn.DefaultTelnetBatchSize, "records per SQL flush")
	batchInterval := flag.Duration("batch-interval", rbn.DefaultTelnetBatchInterval, "maximum delay before flushing a non-empty batch")
	limit := flag.Int("limit", 0, "maximum spots to insert; 0 means no limit")
	dryRun := flag.Bool("dry-run", false, "parse and batch live spots without writing to SQL")
	ctyPath := flag.String("cty-dat", "", "optional CTY.DAT path; defaults to RBN_CTY_DAT or data/cty/cty.dat")
	requireCTY := flag.Bool("require-cty", false, "fail startup if CTY.DAT cannot be loaded")
	qrzEnrich := flag.Bool("qrz-enrich", false, "enable async QRZ cache enrichment for callsigns in committed spot batches")
	qrzQueueSize := flag.Int("qrz-queue-size", 256, "maximum pending QRZ enrichment calls before new calls are dropped")
	qrzWorkers := flag.Int("qrz-workers", 1, "number of async QRZ enrichment workers")
	qrzTimeout := flag.Duration("qrz-timeout", 10*time.Second, "timeout for each QRZ lookup")
	connectTimeout := flag.Duration("connect-timeout", 10*time.Second, "telnet connect timeout")
	flag.Parse()

	if *batchSize <= 0 {
		log.Fatal("-batch-size must be greater than zero")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	lookup, err := loadCallsignLookup(*ctyPath)
	if err != nil {
		if *requireCTY {
			log.Fatal(err)
		}
		log.Printf("cty enrichment disabled: %v", err)
	} else {
		log.Printf("cty enrichment loaded path=%s", lookup.path)
	}

	conn, err := net.DialTimeout("tcp", *rbnAddr, *connectTimeout)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	reader := bufio.NewReader(conn)
	if err := loginToRBN(reader, conn, *loginCall, *connectTimeout); err != nil {
		log.Fatal(err)
	}
	log.Printf("connected rbn_addr=%s login_call=%s dry_run=%v", *rbnAddr, *loginCall, *dryRun)

	var stmt *sql.Stmt
	var db *sql.DB
	var qrzStore qrz.SQLStore
	var qrzEnricher *qrz.AsyncEnricher
	if !*dryRun {
		db, err = sql.Open("mysql", *dsn)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		if err := db.PingContext(ctx); err != nil {
			log.Fatal(err)
		}
		stmt, err = db.PrepareContext(ctx, rbn.SpotInsertSQL)
		if err != nil {
			log.Fatal(err)
		}
		defer stmt.Close()
		qrzStore = qrz.SQLStore{DB: db}
		if *qrzEnrich {
			client, err := qrz.NewClientFromEnv()
			if err != nil {
				log.Fatal(err)
			}
			qrzEnricher, err = qrz.NewAsyncEnricher(
				ctx,
				client,
				qrzStore,
				qrz.WithQueueSize(*qrzQueueSize),
				qrz.WithWorkers(*qrzWorkers),
				qrz.WithLookupTimeout(*qrzTimeout),
				qrz.WithProfileHook(qrzProfileHook(lookup)),
				qrz.WithLogger(log.Printf),
			)
			if err != nil {
				log.Fatal(err)
			}
			defer func() {
				qrzEnricher.Stop()
				log.Printf("qrz enrichment stopped %s", qrzEnricher.Stats())
			}()
			log.Printf("qrz enrichment enabled queue_size=%d workers=%d timeout=%s", *qrzQueueSize, *qrzWorkers, *qrzTimeout)
		}
	} else if *qrzEnrich {
		log.Printf("qrz enrichment disabled in dry-run mode")
	}

	lines := make(chan string, 256)
	readErrs := make(chan error, 1)
	go readLines(reader, lines, readErrs)

	policy := rbn.BatchPolicy{MaxRecords: *batchSize, MaxDelay: *batchInterval}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	var batch []rbn.Spot
	var firstBufferedAt time.Time
	var parsed, ignored, inserted int

	flush := func() error {
		if len(batch) == 0 {
			return nil
		}
		startedAt := time.Now()
		if *dryRun {
			inserted += len(batch)
			log.Printf("dry-run flush records=%d inserted_total=%d", len(batch), inserted)
			batch = batch[:0]
			firstBufferedAt = time.Time{}
			return nil
		}
		if err := ensureActivityBuckets(ctx, db, batch); err != nil {
			return err
		}
		if err := ensureQRZParents(ctx, qrzStore, batch); err != nil {
			return err
		}
		tx, err := db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		txStmt := tx.StmtContext(ctx, stmt)
		for _, spot := range batch {
			if _, err := txStmt.ExecContext(ctx, rbn.SpotSQLArgs(spot)...); err != nil {
				_ = tx.Rollback()
				return err
			}
		}
		if err := tx.Commit(); err != nil {
			return err
		}
		inserted += len(batch)
		enqueueQRZProfiles(qrzEnricher, batch)
		log.Printf("sql flush records=%d inserted_total=%d elapsed=%s", len(batch), inserted, time.Since(startedAt).Round(time.Millisecond))
		batch = batch[:0]
		firstBufferedAt = time.Time{}
		return nil
	}

	for {
		select {
		case <-ctx.Done():
			if err := flush(); err != nil {
				log.Fatal(err)
			}
			log.Printf("stopped parsed=%d ignored=%d inserted=%d", parsed, ignored, inserted)
			return
		case err := <-readErrs:
			if err != nil && !errors.Is(err, net.ErrClosed) {
				log.Printf("telnet read stopped: %v", err)
			}
			if err := flush(); err != nil {
				log.Fatal(err)
			}
			log.Printf("finished parsed=%d ignored=%d inserted=%d", parsed, ignored, inserted)
			return
		case line, ok := <-lines:
			if !ok {
				if err := flush(); err != nil {
					log.Fatal(err)
				}
				log.Printf("finished parsed=%d ignored=%d inserted=%d", parsed, ignored, inserted)
				return
			}
			spot, recognized, err := rbn.ParseTelnetSpot(line, time.Now().UTC(), lookup)
			if err != nil {
				ignored++
				log.Printf("ignore telnet line err=%v line=%q", err, strings.TrimSpace(line))
				continue
			}
			if !recognized {
				ignored++
				continue
			}
			if len(batch) == 0 {
				firstBufferedAt = time.Now()
			}
			batch = append(batch, spot)
			parsed++
			if policy.ShouldFlush(len(batch), firstBufferedAt, time.Now()) {
				if err := flush(); err != nil {
					log.Fatal(err)
				}
			}
			if *limit > 0 && parsed >= *limit {
				if err := flush(); err != nil {
					log.Fatal(err)
				}
				log.Printf("limit reached parsed=%d ignored=%d inserted=%d", parsed, ignored, inserted)
				return
			}
		case now := <-ticker.C:
			if policy.ShouldFlush(len(batch), firstBufferedAt, now) {
				if err := flush(); err != nil {
					log.Fatal(err)
				}
			}
		}
	}
}

func loginToRBN(reader *bufio.Reader, conn net.Conn, call string, timeout time.Duration) error {
	if strings.TrimSpace(call) == "" {
		return nil
	}
	_ = conn.SetDeadline(time.Now().Add(timeout))
	defer conn.SetDeadline(time.Time{})

	var prompt strings.Builder
	for {
		b, err := reader.ReadByte()
		if err != nil {
			return fmt.Errorf("read login prompt: %w", err)
		}
		prompt.WriteByte(b)
		if strings.Contains(prompt.String(), "Please enter your call:") {
			break
		}
	}
	if _, err := fmt.Fprintf(conn, "%s\r\n", strings.TrimSpace(call)); err != nil {
		return fmt.Errorf("write login call: %w", err)
	}
	return nil
}

func readLines(reader *bufio.Reader, lines chan<- string, errs chan<- error) {
	defer close(lines)
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		lines <- scanner.Text()
	}
	errs <- scanner.Err()
}

func ensureQRZParents(ctx context.Context, store qrz.SQLStore, batch []rbn.Spot) error {
	seen := map[string]struct{}{}
	for _, spot := range batch {
		call := strings.ToUpper(strings.TrimSpace(spot.DXCall))
		if call == "" {
			continue
		}
		if _, ok := seen[call]; ok {
			continue
		}
		seen[call] = struct{}{}
		if _, err := store.EnsurePendingProfile(ctx, call); err != nil {
			return err
		}
	}
	return nil
}

func ensureActivityBuckets(ctx context.Context, db *sql.DB, batch []rbn.Spot) error {
	if db == nil {
		return fmt.Errorf("nil sql db")
	}
	seen := map[uint64]struct{}{}
	for _, spot := range batch {
		bucket := rbn.Activity5MBucketFromSpot(spot)
		if bucket.Activity5MID == 0 {
			continue
		}
		if _, ok := seen[bucket.Activity5MID]; ok {
			continue
		}
		seen[bucket.Activity5MID] = struct{}{}
		var existing int64
		err := db.QueryRowContext(ctx,
			`select activity_5m_id from activity_5m_buckets where activity_5m_id = ? limit 1`,
			bucket.Activity5MID,
		).Scan(&existing)
		if err == nil {
			continue
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if _, err := db.ExecContext(ctx, rbn.Activity5MBucketInsertSQL(), rbn.Activity5MBucketSQLArgs(bucket)...); err != nil {
			return err
		}
	}
	return nil
}

func enqueueQRZProfiles(enricher *qrz.AsyncEnricher, batch []rbn.Spot) {
	if enricher == nil {
		return
	}
	for _, spot := range batch {
		enricher.Enqueue(spot.DXCall)
		enricher.Enqueue(spot.SpotterCall)
	}
}

func qrzProfileHook(lookup *callsignLookup) func(*qrz.Profile) {
	return func(profile *qrz.Profile) {
		if lookup == nil || lookup.db == nil || profile == nil || profile.Callsign == "" {
			return
		}
		station, err := lookup.db.Parse(profile.Callsign)
		if err != nil {
			return
		}
		if profile.DXCCPrefix == "" || profile.DXCCPrefix == "UNKNOWN" {
			profile.DXCCPrefix = station.PrimaryPrefix
		}
		if profile.Continent == "" || profile.Continent == "UNKNOWN" {
			profile.Continent = station.Continent
		}
		if profile.CountryName == "" || profile.CountryName == "UNKNOWN" {
			profile.CountryName = station.Country
		}
		if profile.CQZone == 0 {
			profile.CQZone = station.CQZone
		}
		if profile.ITUZone == 0 {
			profile.ITUZone = station.ITUZone
		}
	}
}

type callsignLookup struct {
	db   *callsign.Database
	path string
}

func loadCallsignLookup(path string) (*callsignLookup, error) {
	path = strings.TrimSpace(path)
	var db *callsign.Database
	var err error
	if path != "" {
		db, err = callsign.LoadFile(path)
	} else {
		db, path, err = callsign.LoadDefault()
	}
	if err != nil {
		return nil, err
	}
	return &callsignLookup{db: db, path: path}, nil
}

func (l *callsignLookup) LookupCallsign(call string) (rbn.CallsignInfo, bool) {
	if l == nil || l.db == nil {
		return rbn.CallsignInfo{}, false
	}
	station, err := l.db.Parse(call)
	if err != nil || station.PrimaryPrefix == "" || station.Continent == "" {
		return rbn.CallsignInfo{}, false
	}
	return rbn.CallsignInfo{
		Prefix:    station.PrimaryPrefix,
		Continent: station.Continent,
	}, true
}
