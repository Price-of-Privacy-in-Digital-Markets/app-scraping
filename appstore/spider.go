package appstore

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
	"golang.org/x/sync/errgroup"
	"gopkg.in/guregu/null.v4"
)

type GenreLetter struct {
	Genre    int
	Letter   string
	NextPage int
}

type SpiderProgress struct {
	Genre          int
	Letter         string
	NextPage       null.Int
	DiscoveredApps []AppId
}

func getPageFromUrl(genrePageUrl string) (int64, error) {
	u, err := url.Parse(genrePageUrl)
	if err != nil {
		return 0, err
	}

	query, err := url.ParseQuery(u.RawQuery)
	if err != nil {
		return 0, err
	}

	pageString := query.Get("page")
	page, err := strconv.ParseInt(pageString, 10, 64)
	if err != nil {
		return 0, err
	}

	return page, nil
}

func Spider(ctx context.Context, client *http.Client, progressChan chan<- SpiderProgress, start []GenreLetter) error {
	errgrp, ctx := errgroup.WithContext(ctx)
	connectionLimit := make(chan struct{}, 10)

	for _, genreLetter := range start {
		genre := genreLetter.Genre
		letter := genreLetter.Letter
		page := genreLetter.NextPage

		errgrp.Go(func() error {
			// Initial starting page, this will be redirected to an URL with the prettified genre name
			genrePageUrl := fmt.Sprintf("https://apps.apple.com/us/genre/id%d?letter=%s&page=%d", genre, letter, page)
			for {
				page, err := getPageFromUrl(genrePageUrl)
				if err != nil {
					return err
				}

				// Limit the number of concurrent connections
				select {
				case connectionLimit <- struct{}{}:
				case <-ctx.Done():
					return ctx.Err()
				}

				apps, nextPageUrl, err := scrapeGenrePage(ctx, client, genrePageUrl)
				select {
				case <-connectionLimit:
				case <-ctx.Done():
					return ctx.Err()
				}

				if err != nil {
					return err
				}

				// Update where we have got to in the database
				progress := SpiderProgress{
					Genre:          genre,
					Letter:         letter,
					DiscoveredApps: apps,
				}

				if nextPageUrl != "" {
					progress.NextPage = null.IntFrom(page + 1)
				}

				select {
				case progressChan <- progress:
				case <-ctx.Done():
					return ctx.Err()
				}

				if nextPageUrl == "" {
					return nil
				}

				genrePageUrl = nextPageUrl
			}
		})
	}

	defer close(progressChan)
	return errgrp.Wait()
}

var appUrlPattern *regexp.Regexp = regexp.MustCompile(`^https:\/\/apps.apple.com\/us\/app\/\S+\/id(\d+)$`)

func scrapeGenrePage(ctx context.Context, client *http.Client, genrePageUrl string) (apps []AppId, nextPageUrl string, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", genrePageUrl, nil)
	if err != nil {
		return
	}

	resp, err := client.Do(req)
	if err != nil {
		return
	}

	defer resp.Body.Close()
	z := html.NewTokenizer(resp.Body)

	// Look for two types of links
	// Firstly, links to apps (e.g. https://apps.apple.com/us/app/mythulu-creation-cards/id1442867455)
	// Secondly, links to the next page
	// e.g. <a href="https://apps.apple.com/us/genre/ios-productivity/id6007?letter=M&amp;page=76#page" class="paginate-more">Next</a
TokenLoop:
	for {
		tt := z.Next()
		if tt == html.ErrorToken {
			err = z.Err()
			if err == io.EOF {
				err = nil
			}
			return
		}

		token := z.Token()
		if (token.Type == html.StartTagToken || token.Type == html.SelfClosingTagToken) && token.DataAtom == atom.A {
			nextPageLink := false
			var link string

			for _, a := range token.Attr {
				switch a.Key {
				case "href":
					link = a.Val
				case "class":
					if a.Val == "paginate-more" {
						nextPageLink = true
					}
				}
			}

			// Is this link a link to the next page?
			if nextPageUrl == "" && nextPageLink {
				nextPageUrl = link
				continue TokenLoop
			}

			// Is this a link to an app?
			if matches := appUrlPattern.FindStringSubmatch(link); matches != nil {
				var appId int64
				appId, err = strconv.ParseInt(matches[1], 10, 64)
				if err != nil {
					return
				}
				apps = append(apps, AppId(appId))
			}
		}
	}
}
