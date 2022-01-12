package playstore

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSimilar(t *testing.T) {
	similarApps, err := ScrapeSimilarApps(context.Background(), http.DefaultClient, "bbc.mobile.news.uk", "gb", "en_GB")
	if err != nil {
		t.Fatal(err)
	}

	foundBBCSport := false
	foundSkyNews := false

	for _, similarApp := range similarApps {
		if similarApp.AppId == "uk.co.bbc.android.sportdomestic" {
			foundBBCSport = true
			assert.Zero(t, similarApp.Price)
			assert.Equal(t, similarApp.Developer, "BBC Media App Technologies")
		} else if similarApp.AppId == "com.bskyb.skynews.android" {
			foundSkyNews = true
			assert.Zero(t, similarApp.Price)
			assert.Equal(t, similarApp.Title, "Sky News: Breaking, UK, & World")
			assert.Equal(t, similarApp.Developer, "Sky UK Limited")
		}
	}

	assert.True(t, foundBBCSport)
	assert.True(t, foundSkyNews)
}

func TestSimilarPaid(t *testing.T) {
	similarApps, err := ScrapeSimilarApps(context.Background(), http.DefaultClient, "uk.co.focusmm.DTSCombo", "gb", "en_GB")
	if err != nil {
		t.Fatal(err)
	}

	for _, similarApp := range similarApps {
		if similarApp.AppId == "uk.co.tso.ctt" {
			assert.Positive(t, similarApp.Price)
			assert.Equal(t, similarApp.Currency, "GBP")
		}
	}
}

func TestSimilarNotFound(t *testing.T) {
	similarApps, err := ScrapeSimilarApps(context.Background(), http.DefaultClient, nonExistentAppId, "gb", "en_GB")
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, similarApps, 0)
}
