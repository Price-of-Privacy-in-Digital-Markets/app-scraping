package main

import (
	"context"
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/schollz/progressbar/v3"
	"github.com/spf13/cobra"
	"golang.org/x/sync/errgroup"
	"golang.org/x/time/rate"

	"github.com/Price-of-Privacy-in-Digital-Markets/app-scraping/appstore"
	"github.com/Price-of-Privacy-in-Digital-Markets/app-scraping/internal/database"
)

const (
	DatabaseVersion      uint8 = 2
	country                    = "us"
	language                   = "en"
	NumWorkers                 = 4
	ChunkSize                  = 100
	QueueSize                  = 10_000
	RateLimit                  = 1 * time.Second
	RateLimitedSleepTime       = 60 * time.Second
)

//go:embed schema.sql
var databaseSchema string

func main() {
	ctx, cancel := context.WithCancel(context.Background())

	// Exit on Ctrl-C
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)

	defer func() {
		signal.Stop(signalChan)
		close(signalChan)
	}()

	go func() {
		// First signal: exit gracefully
		select {
		case <-signalChan:
			log.Print("Exiting...")
			cancel()
		case <-ctx.Done():
			return
		}

		// Second signal: force exit
		_, ok := <-signalChan
		if ok {
			os.Exit(1)
		}
	}()

	var databasePath string
	var db *sql.DB

	rootCmd := &cobra.Command{
		Use:   "app_store_scraper",
		Short: "Scrape the Apple App Store",
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			var created bool
			var err error
			db, created, err = database.OpenOrCreate(databasePath, database.DatabaseAppStore, DatabaseVersion)
			if err != nil {
				return err
			}

			if created {
				if _, err := db.ExecContext(ctx, databaseSchema); err != nil {
					return err
				}
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

	spiderCmd := &cobra.Command{
		Use:   "spider",
		Short: "Crawl the App Store to enumerate all the available apps",
		Run: func(cmd *cobra.Command, args []string) {
			if err := spider(ctx, db); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("%+v", err)
			}
		},
	}
	rootCmd.AddCommand(spiderCmd)

	scrapeCmd := &cobra.Command{
		Use: "scrape",
		Run: func(cmd *cobra.Command, args []string) {
			if err := scrape(ctx, db); err != nil && !errors.Is(err, context.Canceled) {
				log.Printf("%+v", err)
			}
		},
	}
	rootCmd.AddCommand(scrapeCmd)

	rootCmd.Execute()
}

// Create an HTTP client with settings for multiple connections to the same hosts
// http://tleyden.github.io/blog/2016/11/21/tuning-the-go-http-client-library-for-load-testing/
func makeHTTPClient() *http.Client {
	retryableClient := retryablehttp.NewClient()
	retryableClient.Logger = nil
	retryableClient.CheckRetry = retryPolicy
	retryableClient.HTTPClient.Timeout = time.Second * 10
	retryableClient.HTTPClient.Transport.(*http.Transport).MaxIdleConns = 100
	retryableClient.HTTPClient.Transport.(*http.Transport).MaxIdleConnsPerHost = 100

	return retryableClient.StandardClient()
}

func retryPolicy(ctx context.Context, resp *http.Response, err error) (bool, error) {
	// Do not retry on 429
	if resp != nil && resp.StatusCode == http.StatusTooManyRequests {
		return false, nil
	}

	shouldRetry, _ := retryablehttp.DefaultRetryPolicy(ctx, resp, err)
	return shouldRetry, nil
}

func makeProgressBar(max int, iterationString string) *progressbar.ProgressBar {
	progress := progressbar.NewOptions(
		max,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString(iterationString),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionSpinnerType(14),
		progressbar.OptionFullWidth(),
		progressbar.OptionUseANSICodes(true),
	)
	progress.RenderBlank()

	return progress
}

func spider(ctx context.Context, db *sql.DB) error {
	spiderStart, err := dbSpiderProgress(ctx, db)
	if err != nil {
		return err
	}

	progress := makeProgressBar(-1, "page")

	statistics := struct {
		AppsFound    int64
		PagesCrawled int64
	}{}
	defer func() {
		log.Printf("Found %d apps after crawling %d pages.", statistics.AppsFound, statistics.PagesCrawled)
	}()

	spiderProgressIn := make(chan appstore.SpiderProgress)
	spiderProgressOut := make(chan appstore.SpiderProgress)

	errgrp, ctx := errgroup.WithContext(ctx)

	errgrp.Go(func() error {
		for spiderProgress := range spiderProgressIn {
			select {
			case <-ctx.Done():
				return ctx.Err()

			case spiderProgressOut <- spiderProgress:
				statistics.AppsFound += int64(len(spiderProgress.DiscoveredApps))
				statistics.PagesCrawled += 1

				progress.Add(1)
			}
		}

		close(spiderProgressOut)
		return nil
	})

	errgrp.Go(func() error {
		return Writer(ctx, db, spiderProgressOut, nil, nil)
	})

	errgrp.Go(func() error {
		client := makeHTTPClient()
		defer client.CloseIdleConnections()
		return appstore.Spider(ctx, client, spiderProgressIn, spiderStart)
	})

	return errgrp.Wait()
}

func scrape(ctx context.Context, db *sql.DB) error {
	total, remaining, err := dbStatistics(ctx, db)
	if err != nil {
		return err
	}

	progress := makeProgressBar(int(total), "apps")
	progress.Set64(total - remaining)

	for {
		errgrp, ctx := errgroup.WithContext(ctx)

		scrapedAppsIn := make(chan []ScrapedApp)
		scrapedAppsOut := make(chan []ScrapedApp)

		notFoundAppsIn := make(chan []appstore.AppId)
		notFoundAppsOut := make(chan []appstore.AppId)

		// Update the progress bar
		errgrp.Go(func() error {
		ProgressLoop:
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()

				case scrapedApps, ok := <-scrapedAppsIn:
					if !ok {
						break ProgressLoop
					}

					select {
					case <-ctx.Done():
						return ctx.Err()

					case scrapedAppsOut <- scrapedApps:
						progress.Add(len(scrapedApps))
						for _, app := range scrapedApps {
							progress.Describe(strconv.Itoa(int(app.AppId)))
						}
					}

				case notFoundApps, ok := <-notFoundAppsIn:
					if !ok {
						break ProgressLoop
					}

					select {
					case <-ctx.Done():
						return ctx.Err()

					case notFoundAppsOut <- notFoundApps:
						progress.Add(len(notFoundApps))
					}
				}
			}

			close(scrapedAppsOut)
			close(notFoundAppsOut)
			return nil
		})

		errgrp.Go(func() error {
			return Writer(ctx, db, nil, scrapedAppsOut, notFoundAppsOut)
		})

		errgrp.Go(func() error {
			client := makeHTTPClient()
			defer client.CloseIdleConnections()

			rateLimiter := rate.NewLimiter(rate.Every(RateLimit), 1)

			// Get the JWT token so we can authenticate against the API
			token, err := appstore.GetToken(ctx, client)
			if err != nil {
				return err
			}

			for {
				// Get apps to scrape
				progress.Describe("Getting apps to scrape")
				appIds, err := appsToScrape(ctx, db, QueueSize)
				if err != nil {
					return err
				}

				if len(appIds) == 0 {
					close(scrapedAppsIn)
					close(notFoundAppsIn)
					return nil
				}

				scrapeErrGrp, scrapeCtx := errgroup.WithContext(ctx)
				toScrape := make(chan []appstore.AppId, NumWorkers)

				// Spawn a goroutine to keep the scrape queue topped up
				scrapeErrGrp.Go(func() error {
					chunks := chunks(appIds, ChunkSize)
					for _, chunk := range chunks {
						select {
						case <-scrapeCtx.Done():
							return scrapeCtx.Err()
						case toScrape <- chunk:
						}
					}

					close(toScrape)
					return nil
				})

				// Spawn a number of scraper goroutines
				for i := 0; i < NumWorkers; i++ {
					scrapeErrGrp.Go(func() error {
						for {
							select {
							case <-scrapeCtx.Done():
								return scrapeCtx.Err()
							case appIds, ok := <-toScrape:
								if !ok {
									return nil
								}

								if err := Scrape(scrapeCtx, client, progress, rateLimiter, token, scrapedAppsIn, notFoundAppsIn, appIds); err != nil {
									// Is this a fatal error or shall we ignore it?

									if errors.Is(err, context.Canceled) {
										return err
									}

									var errNetwork net.Error
									if errors.As(err, &errNetwork) {
										log.Print("Network error: ", errNetwork)
										continue
									}

									return err
								}
							}
						}
					})
				}

				if err := scrapeErrGrp.Wait(); err != nil {
					return err
				}

				// Closing these channels shuts down the other goroutines cleanly
				close(scrapedAppsIn)
				close(notFoundAppsIn)

				return nil
			}
		})

		if err := errgrp.Wait(); err != nil {
			return err
		}
	}

}

func dbStatistics(ctx context.Context, db *sql.DB) (total int64, remaining int64, err error) {
	if err = db.QueryRowContext(ctx, `SELECT COUNT(*) FROM apps`).Scan(&total); err != nil {
		return
	}

	const query = `
	SELECT COUNT(*)
	FROM (
		SELECT
			app_id
		FROM
			apps
		WHERE
			(app_id NOT IN (SELECT app_id FROM scraped_apps)) AND (app_id NOT IN (SELECT app_id FROM not_found_apps))
	)`

	if err = db.QueryRowContext(ctx, query).Scan(&remaining); err != nil {
		return
	}

	return
}

func appsToScrape(ctx context.Context, db *sql.DB, n int) ([]appstore.AppId, error) {
	const query = `
	SELECT
		app_id
	FROM
		apps
	WHERE
		(app_id NOT IN (SELECT app_id FROM scraped_apps)) AND (app_id NOT IN (SELECT app_id FROM not_found_apps))
	LIMIT ?`

	var appIds []appstore.AppId

	rows, err := db.QueryContext(ctx, query, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var appId int64
		if err := rows.Scan(&appId); err != nil {
			return nil, err
		}
		appIds = append(appIds, appstore.AppId(appId))
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return appIds, nil
}

func chunks(xs []appstore.AppId, chunkSize int) [][]appstore.AppId {
	if len(xs) == 0 {
		return nil
	}
	numChunks := (len(xs) + chunkSize - 1) / chunkSize
	divided := make([][]appstore.AppId, 0, numChunks)

	for i := 0; i < len(xs); i += chunkSize {
		end := i + chunkSize

		if end > len(xs) {
			end = len(xs)
		}

		divided = append(divided, xs[i:end])
	}

	return divided
}
