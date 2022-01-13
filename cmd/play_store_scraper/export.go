package main

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"io/ioutil"
	"os"
	"time"

	"github.com/andybalholm/brotli"
)

func Export(ctx context.Context, db *sql.DB, outputPath string) error {
	f, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	if err := export(ctx, db, w); err != nil {
		return err
	}

	if err := w.Flush(); err != nil {
		return err
	}

	return nil
}

func export(ctx context.Context, db *sql.DB, f io.Writer) error {
	type ExportedApp struct {
		ScrapedApp
		ScrapedWhen time.Time `json:"scraped_when"`
	}

	const query = `
	SELECT app_id, scraped_when, data
	FROM scraped_apps
	LEFT JOIN blobs ON scraped_apps.blob_id = blobs.blob_id
	`

	rows, err := db.QueryContext(ctx, query)
	if err != nil {
		return err
	}
	defer rows.Close()

	decompressor := brotli.NewReader(nil)

	encoder := json.NewEncoder(f)
	encoder.SetIndent("", "")
	encoder.SetEscapeHTML(false)

	for rows.Next() {
		var appId string
		var scrapedWhen int64
		var data []byte
		if err := rows.Scan(&appId, &scrapedWhen, &data); err != nil {
			return err
		}

		if err := decompressor.Reset(bytes.NewReader(data)); err != nil {
			return err
		}

		decompressed, err := ioutil.ReadAll(decompressor)
		if err != nil {
			return err
		}

		var scrapedApp ScrapedApp
		if err := json.Unmarshal(decompressed, &scrapedApp); err != nil {
			return err
		}

		exportedApp := ExportedApp{
			ScrapedApp:  scrapedApp,
			ScrapedWhen: time.Unix(scrapedWhen, 0).UTC(),
		}

		if err := encoder.Encode(exportedApp); err != nil {
			return err
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}
	return nil
}
