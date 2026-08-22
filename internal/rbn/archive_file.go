package rbn

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

func OpenArchiveFile(path string) (io.ReadCloser, time.Time, error) {
	fallbackDate := ArchiveDateFromName(path)
	if strings.HasSuffix(strings.ToLower(path), ".zip") {
		zr, err := zip.OpenReader(path)
		if err != nil {
			return nil, time.Time{}, err
		}
		for _, file := range zr.File {
			if strings.HasSuffix(strings.ToLower(file.Name), ".csv") {
				fileDate := ArchiveDateFromName(file.Name)
				if fileDate.IsZero() {
					fileDate = fallbackDate
				}
				rc, err := file.Open()
				if err != nil {
					_ = zr.Close()
					return nil, time.Time{}, err
				}
				return archiveReadCloser{
					Reader: rc,
					close: func() error {
						err1 := rc.Close()
						err2 := zr.Close()
						if err1 != nil {
							return err1
						}
						return err2
					},
				}, fileDate, nil
			}
		}
		_ = zr.Close()
		return nil, time.Time{}, fmt.Errorf("zip archive contains no csv file")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, time.Time{}, err
	}
	return f, fallbackDate, nil
}

func ArchiveDateFromName(path string) time.Time {
	base := filepath.Base(path)
	if len(base) < 8 {
		return time.Time{}
	}
	date, err := time.ParseInLocation("20060102", base[:8], time.UTC)
	if err != nil {
		return time.Time{}
	}
	return date
}

type archiveReadCloser struct {
	io.Reader
	close func() error
}

func (r archiveReadCloser) Close() error {
	return r.close()
}
