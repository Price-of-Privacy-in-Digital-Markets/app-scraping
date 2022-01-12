package main

import (
	"bufio"
	"context"
	"database/sql"
	"os"
	"strings"
)

func Import(ctx context.Context, db *sql.DB, inputFilePath string) error {
	// Read App IDs to scrape
	file, err := os.Open(inputFilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	var appIds []string

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		appId := strings.TrimSpace(scanner.Text())
		appIds = append(appIds, appId)
	}

	if err := scanner.Err(); err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Create prepared statement
	insertApp, err := tx.PrepareContext(ctx, "INSERT INTO apps (app_id) VALUES (?) ON CONFLICT DO NOTHING")
	if err != nil {
		return err
	}
	defer insertApp.Close()

	for _, appId := range appIds {
		if _, err := insertApp.ExecContext(ctx, appId); err != nil {
			return err
		}
	}

	return tx.Commit()
}
