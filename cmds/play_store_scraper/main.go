package main

import (
	"context"
	"database/sql"
	"errors"

	_ "embed"
	"log"
	"os"
	"os/signal"

	"cmds/internal/database"

	"github.com/spf13/cobra"
)

const (
	DatabaseVersion uint8 = 2
	QueueSize       int   = 1_000
)

var scrapeConfig ScrapeConfig = ScrapeConfig{
	Language:                    "en",
	Country:                     "us",
	AdditionalCountriesForPrice: []string{"gb", "de", "fr", "it", "ru", "jp", "in", "br"},
}

//go:embed schema.sql
var databaseSchema string

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	defer func() {
		signal.Stop(signalChan)
		cancel()
	}()

	go func() {
		// First signal
		select {
		case <-signalChan:
			log.Print("Exiting...")
			cancel()
		case <-ctx.Done():
			return
		}

		// Second signal
		_, ok := <-signalChan
		if ok {
			os.Exit(1)
		}
	}()

	var databasePath string
	var db *sql.DB

	rootCmd := &cobra.Command{
		Use:   "play_store_scraper",
		Short: "Scrape the Google Play Store",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var created bool
			var err error
			db, created, err = database.OpenOrCreate(databasePath, database.DatabaseAppStore, DatabaseVersion)
			if err != nil {
				return err
			}

			if created {
				tx, err := db.BeginTx(ctx, nil)
				if err != nil {
					return err
				}
				defer tx.Rollback()

				if _, err := tx.ExecContext(ctx, databaseSchema); err != nil {
					return err
				}

				return tx.Commit()
			}

			return nil
		},
		PersistentPostRunE: func(cmd *cobra.Command, args []string) error {
			if _, err := db.Exec("PRAGMA optimize"); err != nil {
				return err
			}
			if err := db.Close(); err != nil {
				return err
			}

			return nil
		},
	}
	rootCmd.PersistentFlags().StringVar(&databasePath, "database", "", "Path to database")
	rootCmd.MarkPersistentFlagRequired("database")

	importCmd := &cobra.Command{
		Use: "import",
		Run: func(cmd *cobra.Command, args []string) {
			if err := Import(ctx, db, args); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("%+v", err)
			}
		},
		Args: cobra.MinimumNArgs(1),
	}
	importCmd.MarkFlagRequired("input")
	rootCmd.AddCommand(importCmd)

	var numScrapers int

	scrapeCmd := &cobra.Command{
		Use: "scrape",
		Run: func(cmd *cobra.Command, args []string) {
			if err := Scrape(ctx, db, numScrapers); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("%+v", err)
			}
		},
	}
	scrapeCmd.Flags().IntVar(&numScrapers, "num-scrapers", 20, "Number of simultaneous scrapers")
	rootCmd.AddCommand(scrapeCmd)

	rootCmd.Execute()
}
