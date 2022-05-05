package main

import (
	"context"
	"errors"
	"net/http"
	"time"

	"golang.org/x/sync/errgroup"

	"github.com/Price-of-Privacy-in-Digital-Markets/app-scraping/appstore"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/time/rate"
)

type ScrapedApp struct {
	appstore.Details
	PrivacyNutritionLabels appstore.PrivacyNutritionLabels `json:"privacy_nutrition_labels"`
}

func Scrape(ctx context.Context, client *http.Client, progress *progressbar.ProgressBar, rateLimiter *rate.Limiter, token appstore.Token, scrapedAppsChan chan<- []ScrapedApp, notFoundAppsChan chan<- []appstore.AppId, appIds []appstore.AppId) error {
	errgrp, scrapeCtx := errgroup.WithContext(ctx)

	detailsChan := make(chan map[appstore.AppId]appstore.Details, 1)
	privacyChan := make(chan map[appstore.AppId]appstore.PrivacyNutritionLabels, 1)

	errgrp.Go(func() error {
		details, err := appstore.ScrapeDetails(scrapeCtx, client, appIds)
		if err != nil {
			return err
		}

		detailsChan <- details
		return nil
	})

	errgrp.Go(func() error {
	PrivacyLoop:
		for {
			if err := rateLimiter.Wait(scrapeCtx); err != nil {
				return err
			}
			privacy, err := appstore.ScrapePrivacy(scrapeCtx, client, token, appIds)
			if err != nil {
				if errors.Is(err, appstore.ErrRateLimited) {
					progress.Describe("Rate limited")
					rateLimiter.SetLimit(rate.Limit(0))
					rateLimiter.SetLimitAt(time.Now().Add(RateLimitedSleepTime), rate.Every(RateLimit))
					continue PrivacyLoop
				} else {
					return err
				}
			}

			privacyChan <- privacy
			return nil
		}
	})

	if err := errgrp.Wait(); err != nil {
		return err
	}

	appsDetails, appsPrivacy := <-detailsChan, <-privacyChan
	scrapedApps := make([]ScrapedApp, 0, len(appsDetails))
	notFoundApps := make([]appstore.AppId, 0, len(appIds)-len(appsDetails))

	for _, appId := range appIds {
		details, existsDetails := appsDetails[appId]
		privacy, existsPrivacy := appsPrivacy[appId]
		if existsDetails && existsPrivacy {
			scrapedApps = append(scrapedApps, ScrapedApp{Details: details, PrivacyNutritionLabels: privacy})
		} else {
			notFoundApps = append(notFoundApps, appId)
		}
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case scrapedAppsChan <- scrapedApps:
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	case notFoundAppsChan <- notFoundApps:
	}

	return nil
}
