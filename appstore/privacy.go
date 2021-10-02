package appstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
)

type PrivacyNutritionLabels []PrivacyType

type PrivacyType struct {
	// DATA_LINKED_TO_YOU or DATA_USED_TO_TRACK_YOU or DATA_NOT_COLLECTED
	Identifier string `json:"identifier"`

	// Used by DATA_USED_TO_TRACK_YOU
	DataCategories []PrivacyDataCategories `json:"dataCategories"`

	// Used by DATA_LINKED_TO_YOU
	Purposes []PrivacyPurpose `json:"purposes"`
}

type PrivacyDataCategories struct {
	Identifier string   `json:"identifier"`
	DataTypes  []string `json:"dataTypes"`
}

type PrivacyPurpose struct {
	Identifier     string                  `json:"identifier"`
	DataCategories []PrivacyDataCategories `json:"dataCategories"`
}

// If the app ID is not found, then it is not returned in the map.
func ScrapePrivacy(ctx context.Context, client *http.Client, token Token, appIds []AppId) (map[AppId]PrivacyNutritionLabels, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://amp-api.apps.apple.com/v1/catalog/US/apps", nil)
	if err != nil {
		return nil, err
	}

	req.Header.Add("User-Agent", fakeUserAgent)
	req.Header.Add("Origin", "https://apps.apple.com")
	req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))

	// Set the query parameters
	q := req.URL.Query()
	q.Add("platform", "web")
	q.Add("l", "en-us")
	q.Add("ids", commaSeparatedAppIDs(appIds))
	q.Add("extend", "privacyDetails")
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusTooManyRequests {
			return nil, ErrRateLimited
		} else {
			return nil, fmt.Errorf("ScrapePrivacy: %s", resp.Status)
		}
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var response privacyResponse
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, err
	}

	appPrivacyLabels := make(map[AppId]PrivacyNutritionLabels)

	for _, app := range response.Data {
		appPrivacyLabels[app.Id] = PrivacyNutritionLabels(app.Attributes.PrivacyDetails.PrivacyTypes)
	}

	return appPrivacyLabels, nil
}

type privacyResponse struct {
	Data []struct {
		Id         AppId `json:"id,string"`
		Attributes struct {
			PrivacyDetails struct {
				PrivacyTypes []PrivacyType `json:"privacyTypes"`
			} `json:"privacyDetails"`
		} `json:"attributes"`
	} `json:"data"`
}
