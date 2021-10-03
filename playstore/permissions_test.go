package playstore

import (
	"context"
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPermissions(t *testing.T) {
	permissions, err := ScrapePermissions(context.Background(), http.DefaultClient, "com.google.android.GoogleCamera")
	if err != nil {
		t.Fatal(err)
	}

	takePictures := false
	setWallpaper := false

	for _, permission := range permissions {
		if permission.Group == "Camera" && permission.Permission == "take pictures and videos" {
			takePictures = true
		}

		if permission.Group == "Other" && permission.Permission == "set wallpaper" {
			setWallpaper = true
		}
	}

	assert.True(t, takePictures)
	assert.True(t, setWallpaper)
}

func TestPermissionsNotFound(t *testing.T) {
	_, err := ScrapePermissions(context.Background(), http.DefaultClient, nonExistentAppId)
	errNotFound := &AppNotFoundError{}
	assert.ErrorAs(t, err, &errNotFound)
	assert.Equal(t, nonExistentAppId, errNotFound.AppId)
}
