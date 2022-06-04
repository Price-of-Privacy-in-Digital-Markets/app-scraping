package playstore

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDataSafety(t *testing.T) {
	dataSafety, err := ScrapeDataSafety(context.Background(), http.DefaultClient, "com.google.android.googlequicksearchbox")
	if err != nil {
		t.Fatal(err)
	}

	assert.NotNil(t, dataSafety)

	assert.Equal(t, []DataCategory{}, dataSafety.Sharing)

	assert.Contains(t, dataSafety.Collection, DataCategory{
		Name: "Location",
		DataTypes: []DataType{
			{Name: "Approximate location", Optional: false, Purposes: "App functionality, Analytics, Developer communications, Advertising or marketing, Fraud prevention, security, and compliance, Personalization"},
			{Name: "Precise location", Optional: true, Purposes: "App functionality, Analytics, Fraud prevention, security, and compliance, Personalization"},
		},
	})
}

func TestDataSafetyAppDoesNotExist(t *testing.T) {
	dataSafety, err := ScrapeDataSafety(context.Background(), http.DefaultClient, "abcdefghijklmnopqrstuvwxyz")
	assert.Nil(t, dataSafety)
	assert.Equal(t, ErrAppNotFound, err)
}

func TestDataSafetyNoInfoYet(t *testing.T) {
	dataSafety, err := ScrapeDataSafety(context.Background(), http.DefaultClient, "bbc.mobile.news.uk")
	if err != nil {
		t.Fatal(err)
	}

	assert.Nil(t, dataSafety)
}
