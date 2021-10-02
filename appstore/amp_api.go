package appstore

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"

	"golang.org/x/net/html"
)

type AppId int64
type Token string

/// Get the JWT that is used to access Apple's amp-api.apps.apple.com API.
func GetToken(ctx context.Context, client *http.Client) (Token, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://apps.apple.com/us/developer/apple/id284417353", nil)
	if err != nil {
		return "", err
	}
	req.Header.Add("User-Agent", fakeUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer resp.Body.Close()
	z := html.NewTokenizer(resp.Body)

	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			return "", z.Err()
		}

		token := z.Token()
		if (token.Type == html.StartTagToken || token.Type == html.SelfClosingTagToken) && token.Data == "meta" {
			found := false
			for _, a := range token.Attr {
				if a.Key == "name" && a.Val == "web-experience-app/config/environment" {
					found = true
					break
				}
			}

			if found {
				for _, a := range token.Attr {
					if a.Key == "content" {
						// Found the right tag... now try and extract the JWT
						unescaped, err := url.QueryUnescape(a.Val)
						if err != nil {
							return "", err
						}

						type TokenJSON struct {
							Token string `json:"token"`
						}

						type ConfigJSON struct {
							MediaAPI TokenJSON `json:"MEDIA_API"`
						}

						var configJSON ConfigJSON
						err = json.Unmarshal([]byte(unescaped), &configJSON)
						if err != nil {
							return "", err
						}

						return Token(configJSON.MediaAPI.Token), nil
					}
				}
			}

		}
	}
}
