package playstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/markphelps/optional"
	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

var (
	AF_INIT_DATA_CALLBACK  = regexp.MustCompile(`AF_initDataCallback\(\{key:\s*'([a-zA-Z0-9:]+)',.*?data:\s*(.*?),\s*sideChannel:\s*\{\}\}\);`)
	SERVICE_REQUEST_BODY   = regexp.MustCompile(`var AF_dataServiceRequests = \{(.*?)\};\s*var AF_initDataChunkQueue`)
	SERVICE_REQUEST_KEY_ID = regexp.MustCompile(`'(ds:[0-9]+)'\s*:\s*\{.*?id\s*:\s*'([a-zA-Z0-9]+)'.*?\}`)
)

func extractScriptData(body io.Reader) (dataMap map[string]interface{}, serviceRequestMap map[string]string, err error) {
	document, err := html.Parse(body)
	if err != nil {
		return
	}

	// Go through all the nodes and find the script tags
	var scripts []string
	var visitor func(*html.Node)
	visitor = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.Script {
			if n.FirstChild != nil && n.FirstChild.Type == html.TextNode {
				scripts = append(scripts, n.FirstChild.Data)
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visitor(c)
		}
	}
	visitor(document)

	// Mapping of keys, e.g. ds:1, to data
	dataMap = make(map[string]interface{})

	// Sometime the mappings, e.g ds:1, ds:2 change for different countries but the service request IDs appear to be
	// permanent. Create a mapping of service request IDs => keys from the the javascript objects returned by
	// the AF_dataServiceRequests function
	// See: https://github.com/facundoolano/google-play-scraper/pull/412
	serviceRequestMap = make(map[string]string)

	for _, s := range scripts {
		matches := AF_INIT_DATA_CALLBACK.FindAllStringSubmatch(s, -1)
		for _, m := range matches {
			key := m[1]
			var data []interface{}

			d := json.NewDecoder(strings.NewReader(m[2]))
			d.UseNumber()
			err = d.Decode(&data)
			if err != nil {
				return
			}
			dataMap[key] = data
		}

		service_request_match := SERVICE_REQUEST_BODY.FindStringSubmatch(s)
		if service_request_match != nil {
			matches := SERVICE_REQUEST_KEY_ID.FindAllStringSubmatch(service_request_match[1], -1)
			for _, m := range matches {
				serviceRequestMap[m[2]] = m[1]
			}
		}
	}

	return
}

type Details struct {
	AppId                    string           `json:"appId"`
	Country                  string           `json:"country"`
	Language                 string           `json:"language"`
	Title                    string           `json:"title"`
	Description              string           `json:"description"`
	DescriptionHTML          string           `json:"descriptionHTML"`
	Summary                  string           `json:"summary"`
	Installs                 optional.String  `json:"installs"`
	MinInstalls              optional.Int64   `json:"minInstalls"`
	MaxInstalls              optional.Int64   `json:"maxInstalls"`
	Score                    optional.Float64 `json:"score"`
	ScoreText                optional.String  `json:"scoreText"`
	Ratings                  int64            `json:"ratings"`
	Reviews                  int64            `json:"reviews"`
	Histogram                Histogram        `json:"histogram"`
	Price                    float64          `json:"price"`
	Currency                 optional.String  `json:"currency"`
	PriceText                string           `json:"priceText"`
	Sale                     bool             `json:"sale"`
	SaleTime                 OptionalTime     `json:"saleTime"`
	OriginalPrice            optional.Float64 `json:"originalPrice"`
	SaleText                 optional.String  `json:"saleText"`
	Available                bool             `json:"available"`
	OffersIAP                bool             `json:"offersIAP"`
	IAPRange                 optional.String  `json:"IAPRange"`
	Size                     string           `json:"size"`
	AndroidVersion           string           `json:"androidVersion"`
	Developer                string           `json:"developer"`
	DeveloperId              int64            `json:"developerId"`
	DeveloperEmail           optional.String  `json:"developerEmail"`
	DeveloperWebsite         optional.String  `json:"developerWebsite"`
	DeveloperAddress         optional.String  `json:"developerAddress"`
	PrivacyPolicy            optional.String  `json:"privacyPolicy"`
	DeveloperInternalID      string           `json:"developerInternalID"`
	Genre                    string           `json:"genre"`
	GenreId                  string           `json:"genreId"`
	FamilyGenre              optional.String  `json:"familyGenre"`
	FamilyGenreId            optional.String  `json:"familyGenreId"`
	Icon                     string           `json:"icon"`
	HeaderImage              optional.String  `json:"headerImage"`
	Screenshots              []string         `json:"screenshots"`
	Video                    optional.String  `json:"video"`
	VideoImage               optional.String  `json:"videoImage"`
	ContentRating            optional.String  `json:"contentRating"`
	ContentRatingDescription optional.String  `json:"contentRatingDescription"`
	AdSupported              bool             `json:"adSupported"`
	Updated                  time.Time        `json:"updated"`
	Version                  string           `json:"version"`
	RecentChanges            optional.String  `json:"recentChanges"`
	Comments                 []string         `json:"comments"`
	EditorsChoice            bool             `json:"editorsChoice"`
}

type Histogram struct {
	Stars0 json.Number `json:"0"`
	Stars1 json.Number `json:"1"`
	Stars2 json.Number `json:"2"`
	Stars3 json.Number `json:"3"`
	Stars4 json.Number `json:"4"`
	Stars5 json.Number `json:"5"`
}

func ScrapeDetails(ctx context.Context, client *http.Client, appId string, country string, language string) (details Details, err error) {
	req, err := http.NewRequestWithContext(ctx, "GET", "https://play.google.com/store/apps/details", nil)
	if err != nil {
		return
	}
	q := url.Values{}
	q.Set("id", appId)
	q.Set("gl", country)
	q.Set("hl", language)
	req.URL.RawQuery = q.Encode()

	resp, err := client.Do(req)
	if err != nil {
		return
	}

	if resp.StatusCode == 404 {
		err = &AppNotFoundError{appId}
		return
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return
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
		return
	}

	// Now try and extract the data from the JSON blobs
	extract := newExtractor(dataMap, serviceRequestIdMap)

	descriptionHTML := extract.Block("ds:6").String(0, 10, 0, 1)
	description, err := textFromHTML(descriptionHTML)
	if err != nil {
		extract.Error(fmt.Errorf("app description contains invalid HTML"))
	}

	inAppPurchases := extract.Block("ds:6").OptionalString(0, 12, 12, 0)

	details = Details{
		AppId:                    appId,
		Country:                  country,
		Language:                 language,
		Title:                    extract.Block("ds:6").String(0, 0, 0),
		Description:              description,
		DescriptionHTML:          descriptionHTML,
		Summary:                  extract.Block("ds:6").String(0, 10, 1, 1),
		Installs:                 extract.Block("ds:6").OptionalString(0, 12, 9, 0),
		MinInstalls:              extract.Block("ds:6").OptionalInt64(0, 12, 9, 1),
		MaxInstalls:              extract.Block("ds:6").OptionalInt64(0, 12, 9, 2),
		Score:                    extract.Block("ds:7").OptionalFloat64(0, 6, 0, 1),
		ScoreText:                extract.Block("ds:7").OptionalString(0, 6, 0, 0),
		Ratings:                  extract.Block("ds:7").OptionalInt64(0, 6, 2, 1).OrElse(0),
		Reviews:                  extract.Block("ds:7").OptionalInt64(0, 6, 3, 1).OrElse(0),
		Histogram:                histogram(extract.Block("ds:7").Json(0, 6, 1), extract.Error),
		Price:                    price(extract.Block("ds:4").OptionalFloat64(0, 2, 0, 0, 0, 1, 0, 0)),
		Currency:                 extract.Block("ds:4").OptionalString(0, 2, 0, 0, 0, 1, 0, 1),
		PriceText:                priceText(extract.Block("ds:4").OptionalString(0, 2, 0, 0, 0, 1, 0, 2)),
		Sale:                     extract.Block("ds:4").Bool(0, 2, 0, 0, 0, 14, 0, 0),
		SaleTime:                 saleTime(extract.Block("ds:4").OptionalInt64(0, 2, 0, 0, 0, 14, 0, 0)),
		OriginalPrice:            originalPrice(extract.Block("ds:4").OptionalFloat64(0, 2, 0, 0, 0, 1, 1, 0)),
		SaleText:                 extract.Block("ds:4").OptionalString(0, 2, 0, 0, 0, 14, 1),
		Available:                extract.Block("ds:6").Bool(0, 12, 11, 0),
		OffersIAP:                inAppPurchases.Present(),
		IAPRange:                 inAppPurchases,
		Size:                     extract.Block("ds:3").String(0),
		AndroidVersion:           extract.Block("ds:3").String(2),
		Developer:                extract.Block("ds:6").String(0, 12, 5, 1),
		DeveloperId:              extract.Block("ds:6").Int64(0, 12, 5, 0, 0),
		DeveloperEmail:           extract.Block("ds:6").OptionalString(0, 12, 5, 2, 0),
		DeveloperWebsite:         extract.Block("ds:6").OptionalString(0, 12, 5, 3, 5, 2),
		DeveloperAddress:         extract.Block("ds:6").OptionalString(0, 12, 5, 4, 0),
		PrivacyPolicy:            extract.Block("ds:6").OptionalString(0, 12, 7, 2),
		DeveloperInternalID:      extract.Block("ds:6").String(0, 12, 5, 0, 0),
		Genre:                    extract.Block("ds:6").String(0, 12, 13, 0, 0),
		GenreId:                  extract.Block("ds:6").String(0, 12, 13, 0, 2),
		FamilyGenre:              extract.Block("ds:6").OptionalString(0, 12, 13, 1, 0),
		FamilyGenreId:            extract.Block("ds:6").OptionalString(0, 12, 13, 1, 2),
		Icon:                     extract.Block("ds:6").String(0, 12, 1, 3, 2),
		HeaderImage:              extract.Block("ds:6").OptionalString(0, 12, 2, 3, 2),
		Screenshots:              screenshots(extract.Block("ds:6").Json(0, 12, 0), extract.Error),
		Video:                    extract.Block("ds:6").OptionalString(0, 12, 3, 0, 3, 2),
		VideoImage:               extract.Block("ds:6").OptionalString(0, 12, 3, 1, 3, 2),
		ContentRating:            extract.Block("ds:6").OptionalString(0, 12, 4, 0),
		ContentRatingDescription: extract.Block("ds:6").OptionalString(0, 12, 4, 2, 1),
		AdSupported:              extract.Block("ds:6").Bool(0, 12, 14, 0),
		Updated:                  updated(extract.Block("ds:6").Int64(0, 12, 8, 0)),
		Version:                  extract.Block("ds:3").String(1),
		RecentChanges:            extract.Block("ds:6").OptionalString(0, 12, 6, 1),
		Comments:                 comments(extract.BlockWithServiceRequestId("UsvDTd").Json(0), extract.Error),
		EditorsChoice:            extract.Block("ds:6").Bool(0, 12, 15, 0),
	}

	if extract.Errors() != nil {
		err = &DetailsExtractError{
			AppId:    appId,
			Country:  country,
			Language: language,
			Errors:   extract.Errors(),
			Body:     body,
		}
	}

	return
}

type DetailsExtractError struct {
	AppId    string
	Country  string
	Language string
	Errors   []error
	Body     []byte
}

func (e *DetailsExtractError) Error() string {
	sb := strings.Builder{}

	sb.WriteString(fmt.Sprintf("Error extracting data from %s (country: %s, language: %s)\n", e.AppId, e.Country, e.Language))
	for _, err := range e.Errors {
		sb.WriteString(fmt.Sprintf("\t- %s\n", err.Error()))
	}

	return sb.String()
}

func price(maybePrice optional.Float64) float64 {
	return maybePrice.OrElse(0) / 1000000
}

func originalPrice(maybePrice optional.Float64) optional.Float64 {
	price, err := maybePrice.Get()
	if err != nil {
		return optional.Float64{}
	}
	return optional.NewFloat64(price / 1000000)
}

func priceText(maybePriceText optional.String) string {
	priceText, err := maybePriceText.Get()
	if err != nil || priceText == "" {
		return "Free"
	}
	return priceText
}

func histogram(val interface{}, errFunc func(error)) Histogram {
	defer func() {
		if r := recover(); r != nil {
			errFunc(fmt.Errorf("histogram: %v", r))
		}
	}()

	if val == nil {
		return Histogram{}
	}

	return Histogram{
		Stars1: pluckPanic(val, 1, 1).(json.Number),
		Stars2: pluckPanic(val, 2, 1).(json.Number),
		Stars3: pluckPanic(val, 3, 1).(json.Number),
		Stars4: pluckPanic(val, 4, 1).(json.Number),
		Stars5: pluckPanic(val, 5, 1).(json.Number),
	}
}

func developerId(val string, errFunc func(error)) string {
	defer func() {
		if r := recover(); r != nil {
			errFunc(fmt.Errorf("developerId: %v", r))
		}
	}()

	return strings.Split(val, "id=")[1]
}

func screenshots(val interface{}, errFunc func(error)) []string {
	defer func() {
		if r := recover(); r != nil {
			errFunc(fmt.Errorf("screenshots: %v", r))
		}
	}()

	if val == nil {
		return []string{}
	}

	var screenshots []string

	for _, s := range val.([]interface{}) {
		screenshot := pluckPanic(s, 3, 2).(string)
		screenshots = append(screenshots, screenshot)
	}

	return screenshots
}

func comments(val interface{}, errFunc func(error)) []string {
	defer func() {
		if r := recover(); r != nil {
			errFunc(fmt.Errorf("comments: %v", r))
		}
	}()

	if val == nil {
		return []string{}
	}

	const MAX_COMMENTS_TO_EXTRACT = 5
	var comments []string

	s := val.([]interface{})

	for i := 0; i < len(s) && len(comments) < MAX_COMMENTS_TO_EXTRACT; i++ {
		c := s[i]
		if c != nil {
			comment, ok := pluckPanic(c, 4).(string)
			if ok {
				comments = append(comments, comment)
			}
		}
	}

	return comments
}

func updated(val int64) time.Time {
	return time.Unix(val, 0).UTC()
}

func saleTime(val optional.Int64) OptionalTime {
	timestamp, err := val.Get()
	if err != nil {
		return OptionalTime{}
	}
	return NewOptionalTime(time.Unix(timestamp, 0).UTC())
}
