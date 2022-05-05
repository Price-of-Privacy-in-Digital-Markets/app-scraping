package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"

	"github.com/andybalholm/brotli"

	"github.com/Price-of-Privacy-in-Digital-Markets/app-scraping/appstore"
)

func dbSpiderProgress(ctx context.Context, db *sql.DB) ([]appstore.GenreLetter, error) {
	var progress []appstore.GenreLetter

	rows, err := db.QueryContext(ctx, "SELECT genre, letter, page_reached FROM spider_progress WHERE page_reached IS NOT NULL")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			genre    int
			letter   string
			nextPage int
		)

		if err := rows.Scan(&genre, &letter, &nextPage); err != nil {
			return nil, err
		}

		progress = append(progress, appstore.GenreLetter{Genre: genre, Letter: letter, NextPage: nextPage})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return progress, nil
}

func Writer(ctx context.Context, db *sql.DB, spiderProgressChan <-chan appstore.SpiderProgress, scrapedAppsChan <-chan []ScrapedApp, notFoundAppChan <-chan []appstore.AppId) error {
	writer, err := newWriter(ctx, db)
	if err != nil {
		return err
	}
	defer writer.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case spiderProgress, ok := <-spiderProgressChan:
			if !ok {
				return nil
			}

			if err := writer.UpdateSpiderProgress(ctx, spiderProgress); err != nil {
				return err
			}

		case scrapedApps, ok := <-scrapedAppsChan:
			if !ok {
				return nil
			}

			if err := writer.InsertScrapedApps(ctx, scrapedApps); err != nil {
				return err
			}

		case notFoundApps, ok := <-notFoundAppChan:
			if !ok {
				return nil
			}

			if err := writer.InsertNotFoundApps(ctx, notFoundApps); err != nil {
				return err
			}
		}
	}
}

type writer struct {
	db                   *sql.DB
	insertApp            *sql.Stmt
	insertScraped        *sql.Stmt
	insertNotFound       *sql.Stmt
	updateSpiderProgress *sql.Stmt
}

func newWriter(ctx context.Context, db *sql.DB) (*writer, error) {
	insertApp, err := db.PrepareContext(ctx, "INSERT INTO apps (app_id) VALUES (?) ON CONFLICT DO NOTHING")
	if err != nil {
		return nil, err
	}

	insertScraped, err := db.PrepareContext(ctx, "INSERT INTO scraped_apps (app_id, data) VALUES (?, ?)")
	if err != nil {
		return nil, err
	}

	insertNotFound, err := db.PrepareContext(ctx, "INSERT INTO not_found_apps (app_id) VALUES (?)")
	if err != nil {
		return nil, err
	}

	updateSpiderProgress, err := db.PrepareContext(ctx, "UPDATE spider_progress SET page_reached = ? WHERE genre = ? AND letter = ?")
	if err != nil {
		return nil, err
	}

	writer := &writer{
		db:                   db,
		insertApp:            insertApp,
		insertScraped:        insertScraped,
		insertNotFound:       insertNotFound,
		updateSpiderProgress: updateSpiderProgress,
	}

	return writer, nil
}

func (w *writer) Close() error {
	errors := []error{
		w.insertApp.Close(),
		w.insertScraped.Close(),
		w.insertNotFound.Close(),
		w.updateSpiderProgress.Close(),
	}

	for _, err := range errors {
		if err != nil {
			return err
		}
	}

	return nil
}

func (w *writer) UpdateSpiderProgress(ctx context.Context, progress appstore.SpiderProgress) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.StmtContext(ctx, w.updateSpiderProgress).ExecContext(ctx, progress.NextPage, progress.Genre, progress.Letter); err != nil {
		return err
	}

	for _, appId := range progress.DiscoveredApps {
		if _, err := tx.StmtContext(ctx, w.insertApp).ExecContext(ctx, appId); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (w *writer) InsertScrapedApps(ctx context.Context, scrapedApps []ScrapedApp) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, scrapedApp := range scrapedApps {
		uncompressed, err := json.Marshal(scrapedApp)
		if err != nil {
			return err
		}

		compressed := &bytes.Buffer{}
		if err := brotliCompress(compressed, uncompressed); err != nil {
			return err
		}

		if _, err := tx.StmtContext(ctx, w.insertApp).ExecContext(ctx, scrapedApp.AppId); err != nil {
			return err
		}

		if _, err := tx.StmtContext(ctx, w.insertScraped).ExecContext(ctx, scrapedApp.AppId, compressed.Bytes()); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func (w *writer) InsertNotFoundApps(ctx context.Context, notFoundApps []appstore.AppId) error {
	tx, err := w.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for _, appId := range notFoundApps {
		if _, err := tx.StmtContext(ctx, w.insertApp).ExecContext(ctx, appId); err != nil {
			return err
		}

		if _, err := tx.StmtContext(ctx, w.insertNotFound).ExecContext(ctx, appId); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func brotliCompress(dst *bytes.Buffer, src []byte) error {
	compressor := brotli.NewWriterLevel(dst, 9)
	if _, err := compressor.Write(src); err != nil {
		return err
	}
	if err := compressor.Close(); err != nil {
		return err
	}

	return nil
}
