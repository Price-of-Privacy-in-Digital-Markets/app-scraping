package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/sync/errgroup"
	"gopkg.in/guregu/null.v4"

	"github.com/Price-of-Privacy-in-Digital-Markets/app-scraping/playstore"
)

type ScrapedApp struct {
	AppId    string `json:"app_id"`
	Country  string `json:"country"`
	Language string `json:"language"`
	playstore.Details
	SimilarApps []playstore.SimilarApp `json:"similar"`
	DataSafety  *playstore.DataSafety  `json:"data_safety"`
	prices      []PriceInfo
}

type PriceInfo struct {
	Country       string
	Available     bool
	Currency      string
	Price         float64
	OriginalPrice null.Float
}

type ScrapeConfig struct {
	Language                    string
	Country                     string
	AdditionalCountriesForPrice []string
}

func Scrape(ctx context.Context, db *sql.DB, numScrapers int) error {
	// Create HTTP client
	// http://tleyden.github.io/blog/2016/11/21/tuning-the-go-http-client-library-for-load-testing/
	retryableClient := retryablehttp.NewClient()
	retryableClient.Logger = nil
	retryableClient.HTTPClient.Timeout = time.Second * 10
	retryableClient.RetryMax = 10
	retryableClient.HTTPClient.Transport.(*http.Transport).MaxIdleConns = 100
	retryableClient.HTTPClient.Transport.(*http.Transport).MaxIdleConnsPerHost = 100
	defer retryableClient.HTTPClient.CloseIdleConnections()
	client := retryableClient.StandardClient()

	total, remaining, err := dbStatistics(ctx, db)
	if err != nil {
		return err
	}

	progress := progressbar.NewOptions64(
		total,
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionThrottle(100*time.Millisecond),
		progressbar.OptionShowCount(),
		progressbar.OptionShowIts(),
		progressbar.OptionSetItsString("apps"),
		progressbar.OptionOnCompletion(func() {
			fmt.Fprint(os.Stderr, "\n")
		}),
		progressbar.OptionFullWidth(),
		progressbar.OptionUseANSICodes(true),
	)
	progress.RenderBlank()
	progress.Set64(total - remaining)

	for {
		// Get apps to scrape
		total, remaining, err := dbStatistics(ctx, db)
		if err != nil {
			return err
		}
		progress.ChangeMax64(total)

		if remaining == 0 {
			return nil
		}

		appIds, err := appsToScrape(ctx, db, QueueSize)
		if err != nil {
			return err
		}

		scrapedAppIn := make(chan ScrapedApp)
		notFoundAppIn := make(chan string)

		scrapedAppOut := make(chan ScrapedApp)
		notFoundAppOut := make(chan string)

		errgrp, ctx := errgroup.WithContext(ctx)

		// Update the progress bar
		errgrp.Go(func() error {
			for {
				select {
				case <-ctx.Done():
					return ctx.Err()

				case scrapedApp, more := <-scrapedAppIn:
					if !more {
						close(scrapedAppOut)
						return nil
					}
					progress.Add(1)
					select {
					case <-ctx.Done():
						return ctx.Err()
					case scrapedAppOut <- scrapedApp:
					}

				case notFound, more := <-notFoundAppIn:
					if !more {
						close(notFoundAppOut)
						return nil
					}
					progress.Add(1)
					select {
					case <-ctx.Done():
						return ctx.Err()
					case notFoundAppOut <- notFound:
					}
				}
			}
		})

		errgrp.Go(func() error {
			return Writer(ctx, db, scrapedAppOut, notFoundAppOut)
		})

		toScrape := make(chan string, numScrapers)

		errgrp.Go(func() error {
			for _, appId := range appIds {
				select {
				case <-ctx.Done():
					return ctx.Err()
				case toScrape <- appId:
				}
			}
			close(toScrape)
			return nil
		})

		errgrp.Go(func() error {
			errgrp, ctx := errgroup.WithContext(ctx)

			// Spawn a number of worker goroutines
			for i := 0; i < numScrapers; i++ {
				errgrp.Go(func() error {
				MainLoop:
					for {
						select {
						case <-ctx.Done():
							return ctx.Err()
						case appId, ok := <-toScrape:
							if !ok {
								return nil
							}

							if err := ScrapeApp(ctx, client, scrapedAppIn, notFoundAppIn, scrapeConfig, appId); err != nil {
								if errors.Is(err, context.Canceled) {
									return err
								}

								// Is this a fatal error or shall we ignore it?
								var errNetwork net.Error
								if errors.As(err, &errNetwork) {
									log.Print("Network error: ", errNetwork)
									continue MainLoop
								}

								if errors.Is(err, playstore.ErrRateLimited) {
									log.Print(err)
									continue MainLoop
								}

								var errExtractDetails *playstore.DetailsExtractError
								if errors.As(err, &errExtractDetails) {
									fmt.Println(errExtractDetails.Payload)
									log.Print(errExtractDetails)
									continue MainLoop
								}

								var errExtractSimilar *playstore.SimilarAppsExtractError
								if errors.As(err, &errExtractSimilar) {
									log.Print(errExtractSimilar)
									continue MainLoop
								}

								return err
							}
						}
					}
				})
			}

			if err := errgrp.Wait(); err != nil {
				return err
			}

			// Closing these channels shuts down the other goroutines cleanly
			close(scrapedAppIn)
			close(notFoundAppIn)

			return nil
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

func appsToScrape(ctx context.Context, db *sql.DB, n int) ([]string, error) {
	const query = `
	SELECT
		app_id
	FROM
		apps
	WHERE
		(app_id NOT IN (SELECT app_id FROM scraped_apps)) AND (app_id NOT IN (SELECT app_id FROM not_found_apps))
	LIMIT ?`

	var appIds []string

	rows, err := db.QueryContext(ctx, query, n)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var appId string
		if err := rows.Scan(&appId); err != nil {
			return nil, err
		}
		appIds = append(appIds, appId)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return appIds, nil
}

func ScrapeApp(ctx context.Context, client *http.Client, scrapedC chan<- ScrapedApp, notFoundC chan<- string, config ScrapeConfig, appId string) error {
	// Batch requests for details, similar apps and data safety
	requesters := []playstore.BatchRequester{
		playstore.NewDetailsBatchRequester(appId),
		playstore.NewSimilarBatchRequester(appId),
		playstore.NewDataSafetyRequester(appId),
	}

	responses, err := playstore.SendBatchedRequests(ctx, client, config.Country, config.Language, requesters)
	if errors.Is(err, playstore.ErrAppNotFound) {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case notFoundC <- appId:
			return nil
		}
	}
	if err != nil {
		return err
	}

	details := responses[0].(*playstore.Details)
	similar := responses[1].([]playstore.SimilarApp)
	var dataSafety *playstore.DataSafety
	if responses[2] != nil {
		dataSafety = responses[2].(*playstore.DataSafety)
	}

	var prices []PriceInfo

	// Now check if the app is free or paid. If the app is paid (or there is a sale), then scrape price data for additional countries.
	if details.Price > 0 || details.OriginalPrice.ValueOrZero() > 0 {
		prices = make([]PriceInfo, 1+len(config.AdditionalCountriesForPrice))

		// Some apps don't have a valid currency but I think this is only free apps.
		// Error if otherwise
		if details.Available && !details.Currency.Valid {
			return fmt.Errorf("paid app does not have currency: %s", appId)
		}

		// Add price information for the primary country
		prices[0] = PriceInfo{
			Country:       config.Country,
			Available:     details.Available,
			Currency:      details.Currency.String,
			Price:         details.Price,
			OriginalPrice: details.OriginalPrice,
		}

		// Then scrape price information for the additional countries
		errgrp, scrapeCtx := errgroup.WithContext(ctx)
		for i, country := range config.AdditionalCountriesForPrice {
			i, country := i, country
			errgrp.Go(func() error {
				details, err := playstore.ScrapeDetails(scrapeCtx, client, appId, country, config.Language)

				// Sometimes when looking at other countries, the Play Store can report apps as not found
				// (404 error) rather than unavailable.
				if err == playstore.ErrAppNotFound {
					prices[i+1] = PriceInfo{
						Country: country,
					}
					return nil
				}
				if err != nil {
					return err
				}

				// Some apps don't have a valid currency but I think this is only free apps.
				// Return Error if otherwise. Only check if the currency is valid or not if
				// the app was found.
				if details.Available && !details.Currency.Valid {
					return fmt.Errorf("paid app does not have currency: %s", appId)
				}

				prices[i+1] = PriceInfo{
					Country:       country,
					Available:     details.Available,
					Currency:      details.Currency.String,
					Price:         details.Price,
					OriginalPrice: details.OriginalPrice,
				}

				return nil
			})
		}

		if err := errgrp.Wait(); err != nil {
			return err
		}
	}

	scrapedApp := ScrapedApp{
		AppId:       appId,
		Country:     config.Country,
		Language:    config.Language,
		Details:     *details,
		SimilarApps: similar,
		DataSafety:  dataSafety,
		prices:      prices,
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case scrapedC <- scrapedApp:
	}

	return nil
}
