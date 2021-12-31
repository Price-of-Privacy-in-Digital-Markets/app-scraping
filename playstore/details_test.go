package playstore

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"github.com/markphelps/optional"
	"github.com/stretchr/testify/assert"
)

const nonExistentAppId = "This.App.Id.Does.Not.Exist.Hopefully.12345"

func assertPresentAndEqual(t *testing.T, expected interface{}, actual interface{}) {
	switch a := actual.(type) {
	case optional.String:
		value, err := a.Get()
		assert.Nil(t, err)
		assert.Equal(t, expected, value)
	case optional.Bool:
		value, err := a.Get()
		assert.Nil(t, err)
		assert.Equal(t, expected, value)
	case optional.Float64:
		value, err := a.Get()
		assert.Nil(t, err)
		assert.Equal(t, expected, value)
	}
}

func TestNotFound(t *testing.T) {
	_, err := ScrapeDetails(context.Background(), http.DefaultClient, nonExistentAppId, "us", "en")
	errNotFound := &AppNotFoundError{}
	assert.ErrorAs(t, err, &errNotFound)
	assert.Equal(t, nonExistentAppId, errNotFound.AppId)
}

func TestScrapeDetails(t *testing.T) {
	details, err := ScrapeDetails(context.Background(), http.DefaultClient, "com.sgn.pandapop.gp", "us", "en")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "com.sgn.pandapop.gp", details.AppId)
	assert.Equal(t, "us", details.Country)
	assert.Equal(t, "en", details.Language)

	assert.Equal(t, "Bubble Shooter: Panda Pop!", details.Title)
	assert.True(t, details.Available)

	score, err := details.Score.Get()
	assert.Nil(t, err)
	if !(1 <= score && score <= 5) {
		t.Error("Score should be between 1 and 5")
	}

	assert.Equal(t, "GAME_PUZZLE", details.GenreId)
	assert.False(t, details.FamilyGenre.Present())
	assert.False(t, details.FamilyGenreId.Present())

	assert.Equal(t, "4.4 and up", details.AndroidVersion)
	assert.Equal(t, "Free", details.PriceText)
	assert.Equal(t, 0.0, details.Price)
	assert.True(t, details.AdSupported)
	assert.True(t, details.OffersIAP)

	assert.Equal(t, "Jam City, Inc.", details.Developer)
	assert.Equal(t, int64(5509190841173705883), details.DeveloperId)
	assertPresentAndEqual(t, "pandapop@support.jamcity.com", details.DeveloperEmail)
	assertPresentAndEqual(t, "http://www.jamcity.com/privacy", details.PrivacyPolicy)
	assertPresentAndEqual(t, "3652 Eastham Drive\nCulver City, CA 90232", details.DeveloperAddress)
}

func TestPriceText(t *testing.T) {
	details, err := ScrapeDetails(context.Background(), http.DefaultClient, "com.teslacoilsw.launcher.prime", "in", "en")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, fmt.Sprintf("₹%.2f", details.Price), details.PriceText)
	assert.Equal(t, "INR", details.Currency)
}

func TestAvailable(t *testing.T) {
	details, err := ScrapeDetails(context.Background(), http.DefaultClient, "com.jlr.landrover.incontrolremote.appstore", "tr", "en")
	if err != nil {
		t.Fatal(err)
	}

	assert.False(t, details.Available)
}
