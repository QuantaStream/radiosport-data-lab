package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"github.com/QuantaStream/radiosport-data-lab/internal/callsign"
	"github.com/QuantaStream/radiosport-data-lab/internal/qrz"
)

func main() {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)

	dsn := flag.String("mysql-dsn", "MOLIG004@tcp(127.0.0.1:4000)/quanta", "MySQL-compatible DSN for QuantaStream")
	insert := flag.Bool("insert", false, "insert lookup rows into qrz_callsigns")
	ctyPath := flag.String("cty-dat", "", "optional CTY.DAT path; defaults to RBN_CTY_DAT or data/cty/cty.dat")
	requireCTY := flag.Bool("require-cty", false, "fail startup if CTY.DAT cannot be loaded")
	timeout := flag.Duration("timeout", 10*time.Second, "overall lookup timeout")
	flag.Parse()
	calls := flag.Args()
	if len(calls) == 0 {
		log.Fatal("provide at least one callsign")
	}

	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	client, err := qrz.NewClientFromEnv()
	if err != nil {
		log.Fatal(err)
	}

	var lookup *callsign.Database
	if db, path, err := loadCallsignDatabase(*ctyPath); err != nil {
		if *requireCTY {
			log.Fatal(err)
		}
		log.Printf("cty enrichment disabled: %v", err)
	} else {
		lookup = db
		log.Printf("cty enrichment loaded path=%s", path)
	}

	var db *sql.DB
	var store qrz.SQLStore
	if *insert {
		db, err = sql.Open("mysql", *dsn)
		if err != nil {
			log.Fatal(err)
		}
		defer db.Close()
		if err := db.PingContext(ctx); err != nil {
			log.Fatal(err)
		}
		store = qrz.SQLStore{DB: db}
	}

	encoder := json.NewEncoder(os.Stdout)
	var lookedUp, inserted int
	for _, call := range calls {
		call = strings.ToUpper(strings.TrimSpace(call))
		if call == "" {
			continue
		}
		profile, err := client.Lookup(ctx, call)
		if errors.Is(err, qrz.ErrNotFound) {
			profile = qrz.NotFoundProfile(call, time.Now())
		} else if err != nil {
			log.Fatal(err)
		}
		enrichProfile(lookup, &profile)
		if err := encoder.Encode(profile); err != nil {
			log.Fatal(err)
		}
		lookedUp++
		if db != nil {
			if err := store.StoreProfile(ctx, profile); err != nil {
				log.Fatal(err)
			}
			inserted++
		}
	}
	log.Printf("finished looked_up=%d inserted=%d", lookedUp, inserted)
}

func loadCallsignDatabase(path string) (*callsign.Database, string, error) {
	path = strings.TrimSpace(path)
	if path != "" {
		db, err := callsign.LoadFile(path)
		return db, path, err
	}
	return callsign.LoadDefault()
}

func enrichProfile(db *callsign.Database, profile *qrz.Profile) {
	if db == nil || profile == nil || profile.Callsign == "" {
		return
	}
	station, err := db.Parse(profile.Callsign)
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
