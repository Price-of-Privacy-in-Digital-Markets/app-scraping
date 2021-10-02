package appstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strconv"
	"time"
)

type Details struct {
	Id                    AppId     `json:"id"`
	BundleId              string    `json:"bundleId"`
	Title                 string    `json:"title"`
	Url                   string    `json:"url"`
	Description           string    `json:"description"`
	Icon                  string    `json:"icon"`
	Genres                []string  `json:"genres"`
	GenreIds              []int64   `json:"genreIds"`
	PrimaryGenre          string    `json:"primaryGenre"`
	PrimaryGenreId        int64     `json:"primaryGenreId"`
	ContentRating         string    `json:"contentRating"`
	ContentAdvisories     []string  `json:"contentAdvisories"`
	Languages             []string  `json:"languages"`
	Size                  int64     `json:"size"`
	RequiredOsVersion     string    `json:"requiredOsVersion"`
	Released              time.Time `json:"released"`
	Updated               time.Time `json:"updated"`
	ReleaseNotes          string    `json:"releaseNotes"`
	Version               string    `json:"version"`
	Price                 float64   `json:"price"`
	Currency              string    `json:"currency"`
	DeveloperId           int64     `json:"developerId"`
	Developer             string    `json:"developer"`
	DeveloperUrl          string    `json:"developerUrl"`
	DeveloperWebsite      string    `json:"developerWebsite"`
	Score                 float64   `json:"score"`
	Reviews               int64     `json:"reviews"`
	CurrentVersionScore   float64   `json:"currentVersionScore"`
	CurrentVersionReviews int64     `json:"currentVersionReviews"`
	Screenshots           []string  `json:"screenshots"`
	IpadScreenshots       []string  `json:"ipadScreenshots"`
	AppletvScreenshots    []string  `json:"appletvScreenshots"`
	SupportedDevices      []string  `json:"supportedDevices"`
}

func ScrapeDetails(ctx context.Context, client *http.Client, appIds []AppId) (map[AppId]Details, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://itunes.apple.com/lookup", nil)
	if err != nil {
		return nil, err
	}

	q := req.URL.Query()
	q.Add("entity", "software")
	q.Add("id", commaSeparatedAppIDs(appIds))
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, ErrRateLimited
		} else {
			return nil, fmt.Errorf("ScrapeDetails: %s", resp.Status)
		}
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var lookupResponse lookupResponse
	err = json.Unmarshal(body, &lookupResponse)
	if err != nil {
		return nil, err
	}

	// If an ID does not exist, Apple's API just ignores it
	result := make(map[AppId]Details)
	detailsList, err := lookupResponse.ToDetails()
	if err != nil {
		return nil, err
	}
	for _, details := range detailsList {
		result[details.Id] = details
	}

	return result, err
}

func (lr *lookupResponse) ToDetails() ([]Details, error) {
	detailsList := make([]Details, 0, lr.ResultCount)

	for _, result := range lr.Results {
		var icon string
		if result.ArtworkURL512 != "" {
			icon = result.ArtworkURL512
		} else if result.ArtworkURL100 != "" {
			icon = result.ArtworkURL100
		} else {
			icon = result.ArtworkURL60
		}

		genreIds := make([]int64, 0, len(result.GenreIds))
		for _, genreId := range result.GenreIds {
			genreId, err := strconv.ParseInt(genreId, 10, 64)
			if err != nil {
				return nil, err
			}
			genreIds = append(genreIds, genreId)
		}

		details := Details{
			Id:                    AppId(result.TrackID),
			BundleId:              result.BundleID,
			Title:                 result.TrackName,
			Url:                   result.TrackViewURL,
			Description:           result.Description,
			Icon:                  icon,
			Genres:                result.Genres,
			GenreIds:              genreIds,
			PrimaryGenre:          result.PrimaryGenreName,
			PrimaryGenreId:        result.PrimaryGenreID,
			ContentRating:         result.ContentAdvisoryRating,
			ContentAdvisories:     result.Advisories,
			Languages:             result.LanguageCodesISO2A,
			Size:                  result.FileSizeBytes,
			RequiredOsVersion:     result.MinimumOsVersion,
			Released:              result.ReleaseDate,
			Updated:               result.CurrentVersionReleaseDate,
			ReleaseNotes:          result.ReleaseNotes,
			Version:               result.Version,
			Price:                 result.Price,
			Currency:              result.Currency,
			DeveloperId:           result.ArtistID,
			Developer:             result.ArtistName,
			DeveloperUrl:          result.ArtistViewURL,
			DeveloperWebsite:      result.SellerURL,
			Score:                 result.AverageUserRating,
			Reviews:               result.UserRatingCount,
			CurrentVersionScore:   result.AverageUserRatingForCurrentVersion,
			CurrentVersionReviews: result.UserRatingCountForCurrentVersion,
			Screenshots:           result.ScreenshotUrls,
			IpadScreenshots:       result.IpadScreenshotUrls,
			AppletvScreenshots:    result.AppletvScreenshotUrls,
			SupportedDevices:      result.SupportedDevices,
		}
		detailsList = append(detailsList, details)
	}

	return detailsList, nil
}

// (Mostly) automatically generated using https://mholt.github.io/json-to-go/
type lookupResponse struct {
	ResultCount int `json:"resultCount"`
	Results     []struct {
		ScreenshotUrls                     []string      `json:"screenshotUrls"`
		IpadScreenshotUrls                 []string      `json:"ipadScreenshotUrls"`
		AppletvScreenshotUrls              []string      `json:"appletvScreenshotUrls"`
		ArtworkURL60                       string        `json:"artworkUrl60"`
		ArtworkURL512                      string        `json:"artworkUrl512"`
		ArtworkURL100                      string        `json:"artworkUrl100"`
		ArtistViewURL                      string        `json:"artistViewUrl"`
		Features                           []interface{} `json:"features"`
		SupportedDevices                   []string      `json:"supportedDevices"`
		Advisories                         []string      `json:"advisories"`
		IsGameCenterEnabled                bool          `json:"isGameCenterEnabled"`
		Kind                               string        `json:"kind"`
		MinimumOsVersion                   string        `json:"minimumOsVersion"`
		TrackCensoredName                  string        `json:"trackCensoredName"`
		LanguageCodesISO2A                 []string      `json:"languageCodesISO2A"`
		FileSizeBytes                      int64         `json:"fileSizeBytes,string"`
		SellerURL                          string        `json:"sellerUrl"`
		FormattedPrice                     string        `json:"formattedPrice"`
		ContentAdvisoryRating              string        `json:"contentAdvisoryRating"`
		AverageUserRatingForCurrentVersion float64       `json:"averageUserRatingForCurrentVersion"`
		UserRatingCountForCurrentVersion   int64         `json:"userRatingCountForCurrentVersion"`
		AverageUserRating                  float64       `json:"averageUserRating"`
		TrackViewURL                       string        `json:"trackViewUrl"`
		TrackContentRating                 string        `json:"trackContentRating"`
		BundleID                           string        `json:"bundleId"`
		Currency                           string        `json:"currency"`
		TrackID                            int64         `json:"trackId"`
		TrackName                          string        `json:"trackName"`
		ReleaseDate                        time.Time     `json:"releaseDate"`
		SellerName                         string        `json:"sellerName"`
		PrimaryGenreName                   string        `json:"primaryGenreName"`
		GenreIds                           []string      `json:"genreIds"`
		IsVppDeviceBasedLicensingEnabled   bool          `json:"isVppDeviceBasedLicensingEnabled"`
		CurrentVersionReleaseDate          time.Time     `json:"currentVersionReleaseDate"`
		ReleaseNotes                       string        `json:"releaseNotes"`
		PrimaryGenreID                     int64         `json:"primaryGenreId"`
		Description                        string        `json:"description"`
		ArtistID                           int64         `json:"artistId"`
		ArtistName                         string        `json:"artistName"`
		Genres                             []string      `json:"genres"`
		Price                              float64       `json:"price"`
		Version                            string        `json:"version"`
		WrapperType                        string        `json:"wrapperType"`
		UserRatingCount                    int64         `json:"userRatingCount"`
	} `json:"results"`
}
