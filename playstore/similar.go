package playstore

import (
	"bytes"
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"gopkg.in/guregu/null.v4"
)

type SimilarApp struct {
	AppId     string     `json:"app_id"`
	Title     string     `json:"title"`
	Developer string     `json:"developer"`
	Score     null.Float `json:"score"`
	ScoreText string     `json:"score_text"`
	Price     float64    `json:"price"`
	Currency  string     `json:"currency"`
}

func ScrapeSimilarApps(ctx context.Context, client *http.Client, appId string, country string, language string) ([]SimilarApp, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://play.google.com/store/apps/similar", nil)
	if err != nil {
		return nil, err
	}
	q := url.Values{}
	q.Set("id", appId)
	q.Set("gl", country)
	q.Set("hl", language)
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// This does not necessarily mean that the app was not found, just that there are no similar apps
	if resp.StatusCode == 404 {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	dataMap, serviceRequestIdMap, err := extractScriptData(bytes.NewReader(body))
	if err != nil {
		err = &DetailsExtractError{
			AppId:    appId,
			Country:  country,
			Language: language,
			Errors:   []error{err},
			Body:     body,
		}
		return nil, err
	}

	extract := newExtractor(dataMap, serviceRequestIdMap)
	rawSimilarApps, ok := extract.Block("ds:3").Json(0, 1, 0, 0, 0).([]interface{})
	if !ok {
		err := &SimilarAppsExtractError{
			AppId:    appId,
			Country:  country,
			Language: language,
			Errors:   []error{fmt.Errorf("cannot find list of similar apps")},
			Body:     body,
		}
		return nil, err
	}

	var similar []SimilarApp
	for _, rawSimilarApp := range rawSimilarApps {
		appExtract := blockExtractor{
			data:   rawSimilarApp,
			errors: &extract.errors,
		}
		similarApp := SimilarApp{
			AppId:     appExtract.String(12, 0),
			Title:     appExtract.String(2),
			Developer: appExtract.String(4, 0, 0, 0),
			Score:     appExtract.OptionalFloat64(6, 0, 2, 1, 1),
			ScoreText: appExtract.String(6, 0, 2, 1, 0),
			Price:     price(appExtract.OptionalFloat64(7, 0, 3, 2, 1, 0, 0)),
			Currency:  appExtract.String(7, 0, 3, 2, 1, 0, 1),
		}

		similar = append(similar, similarApp)
	}

	if extract.Errors() != nil {
		err = &SimilarAppsExtractError{
			AppId:    appId,
			Country:  country,
			Language: language,
			Errors:   extract.Errors(),
			Body:     body,
		}
	}

	return similar, nil
}

type SimilarAppsExtractError struct {
	AppId    string
	Country  string
	Language string
	Errors   []error
	Body     []byte
}

func (e *SimilarAppsExtractError) Error() string {
	sb := strings.Builder{}

	sb.WriteString(fmt.Sprintf("Error extracting similar apps from %s (country: %s, language: %s)\n", e.AppId, e.Country, e.Language))
	for _, err := range e.Errors {
		sb.WriteString(fmt.Sprintf("\t- %s\n", err.Error()))
	}

	return sb.String()
}
