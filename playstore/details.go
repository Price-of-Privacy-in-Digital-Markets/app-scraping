package playstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math"
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
	AppId                    string
	Country                  string
	Language                 string
	Title                    string
	Description              string
	DescriptionHTML          string
	Summary                  string
	Installs                 string
	MinInstalls              json.Number
	MaxInstalls              json.Number
	Score                    optional.Float64
	ScoreText                optional.String
	Ratings                  int64
	Reviews                  int64
	Histogram                Histogram
	Price                    float64
	Currency                 string
	PriceText                string
	Sale                     bool
	SaleTime                 OptionalTime
	OriginalPrice            optional.Float64
	SaleText                 optional.String
	Available                bool
	OffersIAP                bool
	IAPRange                 optional.String
	Size                     string
	AndroidVersion           string
	Developer                string
	DeveloperId              string
	DeveloperEmail           optional.String
	DeveloperWebsite         optional.String
	DeveloperAddress         optional.String
	PrivacyPolicy            optional.String
	DeveloperInternalID      string
	Genre                    string
	GenreId                  string
	FamilyGenre              optional.String
	FamilyGenreId            optional.String
	Icon                     string
	HeaderImage              optional.String
	Screenshots              []string
	Video                    optional.String
	VideoImage               optional.String
	ContentRating            optional.String
	ContentRatingDescription optional.String
	AdSupported              bool
	Updated                  time.Time
	Version                  string
	RecentChanges            optional.String
	Comments                 []string
	EditorsChoice            bool
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

	descriptionHTML := extract.String("ds:6", 0, 10, 0, 1)
	description, err := textFromHTML(descriptionHTML)
	if err != nil {
		extract.Error(fmt.Errorf("app description contains invalid HTML"))
	}

	inAppPurchases := extract.OptionalString("ds:6", 0, 12, 12, 0)

	details = Details{
		AppId:                    appId,
		Country:                  country,
		Language:                 language,
		Title:                    extract.String("ds:6", 0, 0, 0),
		Description:              description,
		DescriptionHTML:          descriptionHTML,
		Summary:                  extract.String("ds:6", 0, 10, 1, 1),
		Installs:                 extract.String("ds:6", 0, 12, 9, 0),
		MinInstalls:              extract.Number("ds:6", 0, 12, 9, 1),
		MaxInstalls:              extract.Number("ds:6", 0, 12, 9, 2),
		Score:                    extract.OptionalFloat64("ds:7", 0, 6, 0, 1),
		ScoreText:                extract.OptionalString("ds:7", 0, 6, 0, 0),
		Ratings:                  extract.OptionalInt64("ds:7", 0, 6, 2, 1).OrElse(0),
		Reviews:                  extract.OptionalInt64("ds:7", 0, 6, 3, 1).OrElse(0),
		Histogram:                histogram(extract.Json("ds:7", 0, 6, 1), extract.Error),
		Price:                    price(extract.OptionalFloat64("ds:4", 0, 2, 0, 0, 0, 1, 0, 0)),
		Currency:                 extract.String("ds:4", 0, 2, 0, 0, 0, 1, 0, 1),
		PriceText:                priceText(extract.OptionalString("ds:4", 0, 2, 0, 0, 0, 1, 0, 2)),
		Sale:                     extract.Bool("ds:4", 0, 2, 0, 0, 0, 14, 0, 0),
		SaleTime:                 saleTime(extract.OptionalInt64("ds:4", 0, 2, 0, 0, 0, 14, 0, 0)),
		OriginalPrice:            originalPrice(extract.OptionalFloat64("ds:4", 0, 2, 0, 0, 0, 1, 1, 0)),
		SaleText:                 extract.OptionalString("ds:3", 0, 2, 0, 0, 0, 14, 1),
		Available:                extract.Bool("ds:6", 0, 12, 11, 0),
		OffersIAP:                inAppPurchases.Present(),
		IAPRange:                 inAppPurchases,
		Size:                     extract.String("ds:3", 0),
		AndroidVersion:           extract.String("ds:3", 2),
		Developer:                extract.String("ds:6", 0, 12, 5, 1),
		DeveloperId:              developerId(extract.String("ds:6", 0, 12, 5, 5, 4, 2), extract.Error),
		DeveloperEmail:           extract.OptionalString("ds:6", 0, 12, 5, 2, 0),
		DeveloperWebsite:         extract.OptionalString("ds:6", 0, 12, 5, 3, 5, 2),
		DeveloperAddress:         extract.OptionalString("ds:6", 0, 12, 5, 4, 0),
		PrivacyPolicy:            extract.OptionalString("ds:6", 0, 12, 7, 2),
		DeveloperInternalID:      extract.String("ds:6", 0, 12, 5, 0, 0),
		Genre:                    extract.String("ds:6", 0, 12, 13, 0, 0),
		GenreId:                  extract.String("ds:6", 0, 12, 13, 0, 2),
		FamilyGenre:              extract.OptionalString("ds:6", 0, 12, 13, 1, 0),
		FamilyGenreId:            extract.OptionalString("ds:6", 0, 12, 13, 1, 2),
		Icon:                     extract.String("ds:6", 0, 12, 1, 3, 2),
		HeaderImage:              extract.OptionalString("ds:6", 0, 12, 2, 3, 2),
		Screenshots:              screenshots(extract.Json("ds:6", 0, 12, 0), extract.Error),
		Video:                    extract.OptionalString("ds:6", 0, 12, 3, 0, 3, 2),
		VideoImage:               extract.OptionalString("ds:6", 0, 12, 3, 1, 3, 2),
		ContentRating:            extract.OptionalString("ds:6", 0, 12, 4, 0),
		ContentRatingDescription: extract.OptionalString("ds:6", 0, 12, 4, 2, 1),
		AdSupported:              extract.Bool("ds:6", 0, 12, 14, 0),
		Updated:                  updated(extract.Int64("ds:6", 0, 12, 8, 0)),
		Version:                  extract.String("ds:3", 1),
		RecentChanges:            extract.OptionalString("ds:6", 0, 12, 6, 1),
		Comments:                 comments(extract.JsonWithServiceRequestId("UsvDTd", 0), extract.Error),
		EditorsChoice:            extract.Bool("ds:6", 0, 12, 15, 0),
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

type extractor struct {
	dataMap             map[string]interface{}
	serviceRequestIdMap map[string]string
	errors              []error
}

func newExtractor(dataMap map[string]interface{}, serviceRequestIdMap map[string]string) extractor {
	return extractor{
		dataMap:             dataMap,
		serviceRequestIdMap: serviceRequestIdMap,
		errors:              nil,
	}
}

func (e *extractor) Errors() []error {
	return e.errors
}

func (e *extractor) Error(err error) {
	e.errors = append(e.errors, err)
}

func (e *extractor) Json(key string, path ...int) interface{} {
	current := e.dataMap[key]
	ret, err := pluck(current, path...)
	if err != nil {
		return nil
	}
	return ret
}

func (e *extractor) JsonWithServiceRequestId(serviceRequestId string, path ...int) interface{} {
	key, ok := e.serviceRequestIdMap[serviceRequestId]
	if !ok {
		e.Error(fmt.Errorf("extract.JsonWithServiceRequestId(%s, %v): no such service request ID", key, path))
	}
	return e.Json(key, path...)
}

func (e *extractor) Bool(key string, path ...int) bool {
	val := e.Json(key, path...)

	switch val := val.(type) {
	case nil:
		return false
	case bool:
		return val
	case json.Number:
		floating, err := val.Float64()
		if err == nil {
			return floating != 0
		}

		integer, err := val.Int64()
		if err == nil {
			return integer != 0
		}
		e.Error(fmt.Errorf("extract.Bool(%s, %v): cannot convert json.Number to float64 or int64", key, path))
		return false
	case float64, int64:
		return val != 0
	case string:
		return val != ""
	default:
		e.Error(fmt.Errorf("extract.Bool(%s, %v): wrong type", key, path))
		return false
	}
}

func (e *extractor) Number(key string, path ...int) json.Number {
	val := e.Json(key, path...)

	number, ok := val.(json.Number)
	if !ok {
		e.Error(fmt.Errorf("extract.Number(%s, %v): wrong type", key, path))
	}
	return number
}

func (e *extractor) Int64(key string, path ...int) int64 {
	val := e.Json(key, path...)

	switch val := val.(type) {
	case int64:
		return val
	case float64:
		if val == math.Trunc(val) {
			if val > math.MaxInt64 {
				e.Error(fmt.Errorf("extract.Int64(%s, %v): float64 is too large", key, path))
				return 0
			}
			return int64(val)
		} else {
			e.Error(fmt.Errorf("extract.Int64(%s, %v): float64 is not an integer", key, path))
			return 0
		}
	case json.Number:
		integer, err := val.Int64()
		if err != nil {
			e.Error(fmt.Errorf("extract.Int64(%s, %v): cannot convert json.Number to int64", key, path))
		}
		return integer
	default:
		e.Error(fmt.Errorf("extract.Int64(%s, %v): wrong type", key, path))
		return 0
	}
}

func (e *extractor) String(key string, path ...int) string {
	val := e.Json(key, path...)

	switch val := val.(type) {
	case string:
		return val
	default:
		e.Error(fmt.Errorf("extract.String(%s, %v): wrong type", key, path))
		return ""
	}
}

func (e *extractor) OptionalString(key string, path ...int) optional.String {
	val := e.Json(key, path...)

	switch val := val.(type) {
	case nil:
		return optional.String{}
	case string:
		return optional.NewString(val)
	default:
		e.Error(fmt.Errorf("extract.OptionalString(%s, %v): wrong type", key, path))
		return optional.String{}
	}
}

func (e *extractor) OptionalInt64(key string, path ...int) optional.Int64 {
	val := e.Json(key, path...)

	if val == nil {
		return optional.Int64{}
	}
	return optional.NewInt64(e.Int64(key, path...))
}

func (e *extractor) OptionalFloat64(key string, path ...int) optional.Float64 {
	val := e.Json(key, path...)

	switch val := val.(type) {
	case nil:
		return optional.Float64{}
	case json.Number:
		floating, err := val.Float64()
		if err != nil {
			e.Error(fmt.Errorf("cannot convert json.Number to float64"))
			return optional.Float64{}
		}
		return optional.NewFloat64(floating)
	case float64:
		return optional.NewFloat64(val)
	case int64:
		return optional.NewFloat64(float64(val))
	default:
		e.Error(fmt.Errorf("extract.OptionalFloat64(%s, %v): wrong type", key, path))
		return optional.Float64{}
	}
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
	return time.Unix(val, 0)
}

func saleTime(val optional.Int64) OptionalTime {
	timestamp, err := val.Get()
	if err != nil {
		return OptionalTime{}
	}
	return NewOptionalTime(time.Unix(timestamp, 0))
}
