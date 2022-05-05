package main

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"

	"github.com/andybalholm/brotli"
)

type preparedStatements struct {
	InsertApp        *sql.Stmt
	InsertScrapedApp *sql.Stmt
	InsertNotFound   *sql.Stmt
	InsertPrice      *sql.Stmt
}

func Writer(ctx context.Context, db *sql.DB, scrapedC <-chan ScrapedApp, notFoundC <-chan string) error {
	stmts := preparedStatements{}

	insertAppStmt, err := db.PrepareContext(ctx, "INSERT INTO apps (app_id) VALUES (?) ON CONFLICT DO NOTHING")
	if err != nil {
		return err
	}
	defer insertAppStmt.Close()
	stmts.InsertApp = insertAppStmt

	insertScrapedAppStmt, err := db.PrepareContext(ctx, "INSERT INTO scraped_apps (app_id, data) VALUES (?, ?)")
	if err != nil {
		return err
	}
	defer insertScrapedAppStmt.Close()
	stmts.InsertScrapedApp = insertScrapedAppStmt

	insertNotFoundAppStmt, err := db.PrepareContext(ctx, "INSERT INTO not_found_apps (app_id) VALUES (?)")
	if err != nil {
		return err
	}
	defer insertNotFoundAppStmt.Close()
	stmts.InsertNotFound = insertNotFoundAppStmt

	insertPriceStmt, err := db.PrepareContext(ctx, "INSERT INTO prices (app_id, country, currency, price, original_price) VALUES (:app_id, :country, :currency, :price, :original_price)")
	if err != nil {
		return err
	}
	defer insertPriceStmt.Close()
	stmts.InsertPrice = insertPriceStmt

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()

		case scrapedApp, more := <-scrapedC:
			if !more {
				return nil
			}
			if err := insertScrapedApps(ctx, db, &stmts, scrapedApp); err != nil {
				return err
			}

		case notFound, more := <-notFoundC:
			if !more {
				return nil
			}
			if err := insertNotFoundApp(ctx, db, &stmts, notFound); err != nil {
				return err
			}
		}
	}
}

func insertScrapedApps(ctx context.Context, db *sql.DB, stmts *preparedStatements, scrapedApp ScrapedApp) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Try and discover more apps from the similar apps
	if _, err := tx.StmtContext(ctx, stmts.InsertApp).ExecContext(ctx, scrapedApp.AppId); err != nil {
		return err
	}
	for _, similarAppId := range scrapedApp.SimilarApps {
		if _, err := tx.StmtContext(ctx, stmts.InsertApp).ExecContext(ctx, similarAppId.AppId); err != nil {
			return err
		}
	}

	// Convert the scraped app to JSON, compress and insert
	// TODO: this should probably be moved as part of the scraping so it can run in parallel
	uncompressed, err := json.Marshal(scrapedApp)
	if err != nil {
		return err
	}

	compressed := &bytes.Buffer{}
	if err := brotliCompress(compressed, uncompressed); err != nil {
		return err
	}

	if _, err := tx.StmtContext(ctx, stmts.InsertScrapedApp).ExecContext(ctx, scrapedApp.AppId, compressed.Bytes()); err != nil {
		return err
	}

	for _, priceInfo := range scrapedApp.prices {
		// If an app is not available in a country, the price data is meaningless so skip it
		if !priceInfo.Available {
			continue
		}

		if _, err := tx.StmtContext(ctx, stmts.InsertApp).ExecContext(ctx, scrapedApp.AppId); err != nil {
			return err
		}

		args := []interface{}{
			sql.Named("app_id", scrapedApp.AppId),
			sql.Named("country", priceInfo.Country),
			sql.Named("currency", priceInfo.Currency),
			sql.Named("price", priceInfo.Price),
			sql.Named("original_price", priceInfo.OriginalPrice),
		}

		if _, err := tx.StmtContext(ctx, stmts.InsertPrice).ExecContext(ctx, args...); err != nil {
			return err
		}
	}

	return tx.Commit()
}

func insertNotFoundApp(ctx context.Context, db *sql.DB, stmts *preparedStatements, appId string) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.StmtContext(ctx, stmts.InsertApp).ExecContext(ctx, appId); err != nil {
		return err
	}

	if _, err := tx.StmtContext(ctx, stmts.InsertNotFound).ExecContext(ctx, appId); err != nil {
		return err
	}

	return tx.Commit()
}

func brotliCompress(dst *bytes.Buffer, src []byte) error {
	compressor := brotli.NewWriterLevel(dst, 5)
	if _, err := compressor.Write(src); err != nil {
		return err
	}
	if err := compressor.Close(); err != nil {
		return err
	}

	return nil
}
