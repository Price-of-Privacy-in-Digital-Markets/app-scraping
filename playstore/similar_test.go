package playstore

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v4"
)

func TestSimilar(t *testing.T) {
	similarApps, err := ScrapeSimilar(context.Background(), http.DefaultClient, "com.microsoft.office.outlook", "us", "en")
	if err != nil {
		t.Fatal(err)
	}

	foundTeams := false
	foundYahooMail := false

	for _, similarApp := range similarApps {
		if similarApp.AppId == "com.microsoft.teams" {
			foundTeams = true
			assert.Equal(t, null.FloatFrom(0), similarApp.Price)
			assert.Equal(t, similarApp.Title, "Microsoft Teams")
			assert.Equal(t, similarApp.Developer, "Microsoft Corporation")
		} else if similarApp.AppId == "com.yahoo.mobile.client.android.mail" {
			foundYahooMail = true
			assert.Equal(t, null.FloatFrom(0), similarApp.Price)
			assert.Equal(t, similarApp.Title, "Yahoo Mail – Organized Email")
			assert.Equal(t, similarApp.Developer, "Yahoo")
		}
	}

	assert.Len(t, similarApps, 20)
	assert.True(t, foundTeams)
	assert.True(t, foundYahooMail)
}

func TestSimilarPaid(t *testing.T) {
	similarApps, err := ScrapeSimilar(context.Background(), http.DefaultClient, "com.tocaboca.tocahospital", "us", "en")
	if err != nil {
		t.Fatal(err)
	}

	for _, similarApp := range similarApps {
		if similarApp.AppId == "com.tocaboca.tocaneighborhood" {
			assert.True(t, similarApp.Price.Valid)
			assert.Positive(t, similarApp.Price.Float64)
			assert.Equal(t, null.StringFrom("USD"), similarApp.Currency)
		}
	}
}

func TestSimilarNotFound(t *testing.T) {
	similarApps, err := ScrapeSimilar(context.Background(), http.DefaultClient, nonExistentAppId, "us", "en")
	if err != nil && err != ErrAppNotFound {
		t.Fatal(err)
	}

	assert.Equal(t, err, ErrAppNotFound)
	assert.Len(t, similarApps, 0)
}
