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

	"github.com/Price-of-Privacy-in-Digital-Markets/app-scraping/playstore"
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
		playstore.Details
		ScrapedWhen time.Time              `json:"scraped_when"`
		SimilarApps []string               `json:"similar"`
		Permissions []playstore.Permission `json:"permissions"`
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

		similarAppIds := make([]string, 0, len(scrapedApp.SimilarApps))
		for _, similarApp := range scrapedApp.SimilarApps {
			similarAppIds = append(similarAppIds, similarApp.AppId)
		}

		exportedApp := ExportedApp{
			Details:     scrapedApp.Details,
			Permissions: scrapedApp.Permissions,
			SimilarApps: similarAppIds,
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
