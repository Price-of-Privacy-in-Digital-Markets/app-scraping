package playstore

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDataSafety(t *testing.T) {
	dataSafety, err := ScrapeDataSafety(context.Background(), http.DefaultClient, "com.google.android.gm")
	if err != nil {
		t.Fatal(err)
	}

	assert.NotNil(t, dataSafety)

	assert.Equal(t, []DataCategory{}, dataSafety.Sharing)
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
