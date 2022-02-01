package main

import (
	"bufio"
	"context"
	"database/sql"
	"log"
	"os"
	"strings"
)

func Import(ctx context.Context, db *sql.DB, inputFilePaths []string) error {
	var total int64

	for _, inputFile := range inputFilePaths {
		n, err := importAppIds(ctx, db, inputFile)
		if err != nil {
			return err
		}

		total += n
	}

	log.Printf("Imported %d apps.", total)
	return nil
}

func importAppIds(ctx context.Context, db *sql.DB, inputFilePath string) (int64, error) {
	// Read App IDs to scrape
	file, err := os.Open(inputFilePath)
	if err != nil {
		return 0, err
	}
	defer file.Close()

	// Prepare database for inserting app IDs
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()

	insertApp, err := tx.PrepareContext(ctx, "INSERT INTO apps (app_id) VALUES (?) ON CONFLICT DO NOTHING")
	if err != nil {
		return 0, err
	}
	defer insertApp.Close()

	var n int64
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		appId := strings.TrimSpace(scanner.Text())

		if _, err := insertApp.ExecContext(ctx, appId); err != nil {
			return 0, err
		}

		n++
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, err
	}

	return n, nil
}
