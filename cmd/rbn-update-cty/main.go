package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/QuantaStream/radiosport-data-lab/internal/callsign"
)

func main() {
	log.SetFlags(0)

	url := flag.String("url", "https://www.country-files.com/bigcty/cty.dat", "CTY.DAT download URL")
	dest := flag.String("dest", "data/cty/cty.dat", "destination path")
	timeout := flag.Duration("timeout", 30*time.Second, "download timeout")
	flag.Parse()

	if strings.TrimSpace(*dest) == "" {
		log.Fatal("-dest must not be empty")
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, *url, nil)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Fatalf("download %s: %s", *url, resp.Status)
	}

	if err := os.MkdirAll(filepath.Dir(*dest), 0755); err != nil {
		log.Fatal(err)
	}
	tmp := *dest + ".tmp"
	file, err := os.Create(tmp)
	if err != nil {
		log.Fatal(err)
	}
	if _, err := io.Copy(file, resp.Body); err != nil {
		_ = file.Close()
		_ = os.Remove(tmp)
		log.Fatal(err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(tmp)
		log.Fatal(err)
	}

	db, err := callsign.LoadFile(tmp)
	if err != nil {
		_ = os.Remove(tmp)
		log.Fatalf("downloaded file is not a valid CTY.DAT: %v", err)
	}
	if _, err := db.Parse("K1ABC"); err != nil {
		_ = os.Remove(tmp)
		log.Fatalf("downloaded CTY.DAT failed smoke parse: %v", err)
	}
	if err := os.Rename(tmp, *dest); err != nil {
		_ = os.Remove(tmp)
		log.Fatal(err)
	}
	fmt.Printf("updated %s\n", *dest)
}
