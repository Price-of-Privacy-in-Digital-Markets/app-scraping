package playstore

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"gopkg.in/guregu/null.v4"
)

const nonExistentAppId = "This.App.Id.Does.Not.Exist.Hopefully.12345"

func TestNotFound(t *testing.T) {
	_, err := ScrapeDetails(context.Background(), http.DefaultClient, nonExistentAppId, "us", "en")
	assert.ErrorIs(t, err, ErrAppNotFound)
}

func TestScrapeDetails(t *testing.T) {
	details, err := ScrapeDetails(context.Background(), http.DefaultClient, "com.sgn.pandapop.gp", "us", "en")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, "com.sgn.pandapop.gp", details.AppId)
	// assert.Equal(t, "us", details.Country)

	assert.True(t, details.Available)

	assert.Equal(t, "Bubble Shooter: Panda Pop!", details.Title)
	assert.Equal(t, "Match 3: Shoot &amp; Blast Bubbles", details.Summary)
	assert.Equal(t, null.TimeFrom(time.Date(2014, time.January, 10, 4, 15, 29, 0, time.Local)), details.Released)

	assert.Equal(t, "50,000,000+", details.Installs)
	assert.Positive(t, details.MinInstalls)
	assert.Positive(t, details.MaxInstalls)

	assert.True(t, details.Score.Valid)
	if !(1 <= details.Score.Float64 && details.Score.Float64 <= 5) {
		t.Error("Score should be between 1 and 5")
	}
	assert.Equal(t, null.StringFrom(fmt.Sprintf("%1.1f", details.Score.Float64)), details.ScoreText)
	assert.Positive(t, details.Reviews)
	assert.Positive(t, details.Ratings)
	assert.Positive(t, details.Histogram.Stars1)
	assert.Positive(t, details.Histogram.Stars2)
	assert.Positive(t, details.Histogram.Stars3)
	assert.Positive(t, details.Histogram.Stars4)
	assert.Positive(t, details.Histogram.Stars5)

	assert.Equal(t, null.StringFrom("Everyone"), details.ContentRating)
	assert.Equal(t, null.String{}, details.TeacherApprovedAge)

	assert.Equal(t, "GAME_PUZZLE", details.Genre)

	assert.Equal(t, null.StringFrom("7.0"), details.MinAndroidVersion)
	assert.Equal(t, null.IntFrom(24), details.MinAPILevel)
	assert.Equal(t, "Free", details.PriceText)
	assert.Equal(t, 0.0, details.Price)
	assert.True(t, details.AdSupported)
	assert.True(t, details.OffersIAP)
	assert.Equal(t, null.StringFrom("$0.99 - $99.99 per item"), details.IAPRange)
	assert.True(t, details.Available)

	assert.Equal(t, "Jam City, Inc.", details.Developer)
	assert.Equal(t, "5509190841173705883", details.DeveloperId)
	assert.Equal(t, null.StringFrom("pandapop@support.jamcity.com"), details.DeveloperEmail)
	assert.Equal(t, null.StringFrom("http://www.jamcity.com/privacy"), details.PrivacyPolicy)
	assert.Equal(t, null.StringFrom("3652 Eastham Drive\nCulver City, CA 90232"), details.DeveloperAddress)
	assert.Equal(t, null.StringFrom("http://www.jamcity.com/privacy"), details.PrivacyPolicy)

	assert.True(t, details.Icon.Valid)
	assert.NotEqual(t, "", details.Icon.String)
	assert.True(t, details.HeaderImage.Valid)
	assert.NotEqual(t, "", details.HeaderImage.String)
	assert.True(t, details.Video.Valid)
	assert.NotEqual(t, "", details.Video.String)
	assert.True(t, details.VideoImage.Valid)
	assert.NotEqual(t, "", details.VideoImage.String)
	assert.NotEmpty(t, details.Screenshots)

	assert.Contains(t, details.Permissions, Permission{Group: "Phone", Permission: "read phone status and identity"})
	assert.Contains(t, details.Permissions, Permission{Group: "Device ID & call information", Permission: "read phone status and identity"})
	assert.Contains(t, details.Permissions, Permission{Group: "Photos/Media/Files", Permission: "modify or delete the contents of your USB storage"})
	assert.Contains(t, details.Permissions, Permission{Group: "Photos/Media/Files", Permission: "read the contents of your USB storage"})
	assert.Contains(t, details.Permissions, Permission{Group: "Storage", Permission: "modify or delete the contents of your USB storage"})
	assert.Contains(t, details.Permissions, Permission{Group: "Storage", Permission: "read the contents of your USB storage"})

	assert.Contains(t, details.Permissions, Permission{Group: "Other", Permission: "full network access"})
	assert.Contains(t, details.Permissions, Permission{Group: "Other", Permission: "prevent device from sleeping"})
	assert.Contains(t, details.Permissions, Permission{Group: "Other", Permission: "view network connections"})
	assert.Contains(t, details.Permissions, Permission{Group: "Other", Permission: "run at startup"})

	assert.Contains(t, details.Permissions, Permission{Group: "Other", Permission: "receive data from Internet"})
	assert.Contains(t, details.Permissions, Permission{Group: "Other", Permission: "download files without notification"})
}

func TestDetails2(t *testing.T) {
	// com.tocaboca.tocakitchen2
	details, err := ScrapeDetails(context.Background(), http.DefaultClient, "com.tocaboca.tocakitchen2", "us", "en")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, null.StringFrom("Content is generally suitable for all ages. May contain minimal cartoon, fantasy or mild violence and/or infrequent use of mild language."), details.ContentRatingDescription)

	assert.Equal(t, "GAME_EDUCATIONAL", details.Genre)
	assert.Contains(t, details.AdditionalGenres, "GAME_SIMULATION")
	assert.Contains(t, details.AdditionalGenres, "GAME_CASUAL")
	assert.Equal(t, null.StringFrom("6-8"), details.TeacherApprovedAge)

	assert.Contains(t, details.Permissions, Permission{Group: "Wi-Fi connection information", Permission: "view Wi-Fi connections"})
	assert.Contains(t, details.Permissions, Permission{Group: "Storage", Permission: "read the contents of your USB storage"})
	assert.Contains(t, details.Permissions, Permission{Group: "Storage", Permission: "modify or delete the contents of your USB storage"})
	assert.Contains(t, details.Permissions, Permission{Group: "Photos/Media/Files", Permission: "read the contents of your USB storage"})
	assert.Contains(t, details.Permissions, Permission{Group: "Photos/Media/Files", Permission: "modify or delete the contents of your USB storage"})
	assert.Contains(t, details.Permissions, Permission{Group: "Other", Permission: "Google Play license check"})
	assert.Contains(t, details.Permissions, Permission{Group: "Other", Permission: "full network access"})
}

func TestPriceText(t *testing.T) {
	details, err := ScrapeDetails(context.Background(), http.DefaultClient, "com.teslacoilsw.launcher.prime", "in", "en")
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, fmt.Sprintf("₹%.2f", details.Price), details.PriceText)
	assert.Equal(t, null.StringFrom("INR"), details.Currency)
}

func TestAvailable(t *testing.T) {
	// BBC News UK is available in the UK...
	details, err := ScrapeDetails(context.Background(), http.DefaultClient, "bbc.mobile.news.uk", "gb", "en")
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, details.Available)

	// ...but not in the US
	details, err = ScrapeDetails(context.Background(), http.DefaultClient, "bbc.mobile.news.uk", "us", "en")
	if err != nil {
		t.Fatal(err)
	}

	assert.False(t, details.Available)
}

func TestPermissions(t *testing.T) {
	details, err := ScrapeDetails(context.Background(), http.DefaultClient, "com.google.android.GoogleCamera", "in", "en")
	if err != nil {
		t.Fatal(err)
	}

	assert.Contains(t, details.Permissions, Permission{Group: "Camera", Permission: "take pictures and videos"})
	assert.Contains(t, details.Permissions, Permission{Group: "Other", Permission: "set wallpaper"})
}
