package playstore

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	"gopkg.in/guregu/null.v4"
)

type Details struct {
	Title                    string       `json:"title"`
	Description              string       `json:"description"`
	DescriptionHTML          string       `json:"description_html"`
	Summary                  null.String  `json:"summary"`
	Installs                 null.String  `json:"installs"`
	MinInstalls              null.Int     `json:"min_installs"`
	MaxInstalls              null.Int     `json:"max_installs"`
	Score                    null.Float   `json:"score"`
	ScoreText                null.String  `json:"score_text"`
	Ratings                  int64        `json:"ratings"`
	Reviews                  int64        `json:"reviews"`
	Histogram                Histogram    `json:"histogram"`
	Price                    float64      `json:"price"`
	Currency                 null.String  `json:"currency"`
	PriceText                string       `json:"price_text"`
	SaleEndTime              null.Time    `json:"sale_end_time"`
	OriginalPrice            null.Float   `json:"original_price"`
	OriginalPriceText        null.String  `json:"original_price_text"`
	SaleText                 null.String  `json:"sale_text"`
	Available                bool         `json:"available"`
	OffersIAP                bool         `json:"in_app_purchases"`
	IAPRange                 null.String  `json:"in_app_purchases_range"`
	Size                     string       `json:"size"`
	MinAPILevel              null.Int     `json:"min_api"`
	TargetAPILevel           null.Int     `json:"target_api"`
	MinAndroidVersion        null.String  `json:"min_android_version"`
	Developer                string       `json:"developer"`
	DeveloperId              string       `json:"developer_id"`
	DeveloperEmail           null.String  `json:"developer_email"`
	DeveloperWebsite         null.String  `json:"developer_website"`
	DeveloperAddress         null.String  `json:"developer_address"`
	PrivacyPolicy            null.String  `json:"privacy_policy"`
	Genre                    string       `json:"genre_id"`
	AdditionalGenres         []string     `json:"additional_genre_ids"`
	TeacherApprovedAge       null.String  `json:"teacher_approved_age"`
	Icon                     null.String  `json:"icon"`
	HeaderImage              null.String  `json:"header_image"`
	Screenshots              []string     `json:"screenshots"`
	Video                    null.String  `json:"video"`
	VideoImage               null.String  `json:"video_image"`
	ContentRating            null.String  `json:"content_rating"`
	ContentRatingDescription null.String  `json:"content_rating_description"`
	AdSupported              bool         `json:"ad_supported"`
	Released                 null.Time    `json:"released"`
	Updated                  time.Time    `json:"updated"`
	Version                  null.String  `json:"version"`
	RecentChanges            null.String  `json:"recent_changes"`
	RecentChangesTime        null.Time    `json:"recent_changes_time"`
	Permissions              []Permission `json:"permissions"`
}

type Histogram struct {
	Stars1 int64 `json:"1"`
	Stars2 int64 `json:"2"`
	Stars3 int64 `json:"3"`
	Stars4 int64 `json:"4"`
	Stars5 int64 `json:"5"`
}

type Permission struct {
	Group      string `json:"group"`
	Permission string `json:"permission"`
}

type DetailsBatchRequester string

type detailsBatchRequester struct {
	AppId string
}

func NewDetailsBatchRequester(appId string) *detailsBatchRequester {
	return &detailsBatchRequester{AppId: appId}
}

func (br *detailsBatchRequester) BatchRequest() batchRequest {
	return batchRequest{
		RpcId:   "Ws7gDc",
		Payload: fmt.Sprintf(`[null,null,[[1,9,10,11,14,19,20,43,45,47,49,52,58,59,63,69,70,73,74,75,78,79,80,91,92,95,96,97,100,101,103,106,112,119,139,141,145,146]],[[[true],null,[[[]]],null,null,null,null,[null,2],null,null,null,null,null,null,[1],null,null,null,null,null,null,null,[1]],[null,[[[]]]],[null,[[[]]],null,[true]],[null,[[[]]]],null,null,null,null,[[[[]]]],[[[[]]]]],null,[["%s",7]]]`, br.AppId),
	}
}

func (br *detailsBatchRequester) ParseEnvelope(payload string) (interface{}, error) {
	if payload == "" {
		return nil, ErrAppNotFound
	}

	extract := NewExtractor(payload)

	descriptionHTML := extract.String("1.2.72.0.1")
	description, err := textFromHTML(descriptionHTML)
	if err != nil {
		extract.Error(fmt.Errorf("app description contains invalid HTML"))
	}

	iap := extract.OptionalString("1.2.19.0")

	permissions, err := extractPermissions(extract.Json("1.2.74.2"))
	if err != nil {
		extract.Error(fmt.Errorf("permissions: %w", err))
	}

	details := Details{
		Title:           extract.String("1.2.0.0"),
		Description:     description,
		DescriptionHTML: descriptionHTML,
		Summary:         extract.OptionalString("1.2.73.0.1"),
		Installs:        extract.OptionalString("1.2.13.0"),
		MinInstalls:     extract.OptionalInt("1.2.13.1"),
		MaxInstalls:     extract.OptionalInt("1.2.13.2"),
		Score:           extract.OptionalFloat("1.2.51.0.1"),
		ScoreText:       extract.OptionalString("1.2.51.0.0"),
		Ratings:         extract.OptionalInt("1.2.51.2.1").ValueOrZero(),
		Reviews:         extract.OptionalInt("1.2.51.3.1").ValueOrZero(),
		Histogram: Histogram{
			Stars1: extract.OptionalInt("1.2.51.1.1.1").ValueOrZero(),
			Stars2: extract.OptionalInt("1.2.51.1.2.1").ValueOrZero(),
			Stars3: extract.OptionalInt("1.2.51.1.3.1").ValueOrZero(),
			Stars4: extract.OptionalInt("1.2.51.1.4.1").ValueOrZero(),
			Stars5: extract.OptionalInt("1.2.51.1.5.1").ValueOrZero(),
		},
		Price:                    price(extract.OptionalFloat("1.2.57.0.0.0.0.1.0.0").ValueOrZero()),
		Currency:                 extract.OptionalString("1.2.57.0.0.0.0.1.0.1"),
		PriceText:                priceText(extract.OptionalString("1.2.57.0.0.0.0.1.0.2").ValueOrZero()),
		SaleEndTime:              extract.OptionalTime("1.2.57.0.0.0.0.14.0.0"),
		OriginalPrice:            maybePrice(extract.OptionalFloat("1.2.57.0.0.0.0.1.1.0")),
		OriginalPriceText:        extract.OptionalString("1.2.57.0.0.0.0.1.1.2"),
		Available:                extract.Int("1.2.42.0") == 1, // This seems to be 3 on countries where apps are not available, e.g. iPlayer outside UK
		OffersIAP:                iap.Valid && iap.String != "",
		IAPRange:                 iap,
		MinAPILevel:              extract.OptionalInt("1.2.140.1.1.0.0.0"),
		TargetAPILevel:           extract.OptionalInt("1.2.140.1.0.0.0"),
		MinAndroidVersion:        extract.OptionalString("1.2.140.1.1.0.0.1"),
		Developer:                extract.String("1.2.68.0"),
		DeveloperId:              developerId(extract, "1.2.68.1.4.2"),
		DeveloperEmail:           extract.OptionalString("1.2.69.1.0"),
		DeveloperWebsite:         extract.OptionalString("1.2.69.0.5.2"),
		DeveloperAddress:         extract.OptionalString("1.2.69.2.0"),
		PrivacyPolicy:            extract.OptionalString("1.2.99.0.5.2"),
		Genre:                    extract.String("1.2.79.0.0.2"),
		AdditionalGenres:         extract.OptionalStringSlice("1.2.118.#.0.0.2"),
		TeacherApprovedAge:       extract.OptionalString("1.2.111.1"),
		Icon:                     extract.OptionalString("1.2.95.0.3.2"),
		HeaderImage:              extract.OptionalString("1.2.96.0.3.2"),
		Screenshots:              extract.OptionalStringSlice("1.2.78.0.#.3.2"),
		Video:                    extract.OptionalString("1.2.100.0.0.3.2"),
		VideoImage:               extract.OptionalString("1.2.100.0.1.3.2"),
		ContentRating:            extract.OptionalString("1.2.9.0"),
		ContentRatingDescription: extract.OptionalString("1.2.9.6.1"),
		AdSupported:              !extract.IsNull("1.2.48.0"),
		Released:                 extract.OptionalTime("1.2.10.1.0"),
		Updated:                  extract.Time("1.2.145.0.1.0"),
		Version:                  extract.OptionalString("1.2.140.0.0.0"),
		RecentChanges:            extract.OptionalString("1.2.144.1.1"),
		RecentChangesTime:        extract.OptionalTime("1.2.144.2.0"),
		Permissions:              permissions,
	}

	if extract.Errors() != nil {
		err = &DetailsExtractError{
			Errors:  extract.Errors(),
			Payload: payload,
		}
		return nil, err
	}

	return &details, nil
}

type DetailsExtractError struct {
	Errors  []error
	Payload string
}

func (e *DetailsExtractError) Error() string {
	sb := strings.Builder{}

	sb.WriteString("Error extracting details apps:\n")
	for _, err := range e.Errors {
		sb.WriteString(fmt.Sprintf("\t- %s\n", err.Error()))
	}

	return sb.String()
}

var developerIdRe = regexp.MustCompile(`^/store/apps/developer\?id=(.*)$`)

// There are two types of developer ID:
// /store/apps/dev?id=5509190841173705883
// /store/apps/developer?id=TeslaCoil+Software
func developerId(e *extractor, path string) string {
	devUrl := e.String(path)

	if devUrl == "" {
		e.Error(fmt.Errorf("invalid dev url '%s'", devUrl))
		return ""
	}

	// For some stupid reason, the Play Store allows semicolons and other characters in
	// the ID parameter, e.g. /store/apps/developer?id=Prodev;+My+Pro+Apps, so we can't
	// use Go's url parsing but have to use a regex instead.
	matches := developerIdRe.FindStringSubmatch(devUrl)
	if matches != nil {
		return matches[1]
	}

	u, err := url.Parse(devUrl)
	if err != nil {
		e.Error(fmt.Errorf("invalid dev url '%s'", devUrl))
		return ""
	}

	if u.Path == "/store/apps/dev" {
		id := u.Query().Get("id")
		if id != "" {
			return id
		}
	}

	e.Error(fmt.Errorf("invalid dev url '%s'", devUrl))
	return ""
}

func price(p float64) float64 {
	return p / 1000000
}

func maybePrice(maybePrice null.Float) null.Float {
	return null.NewFloat(maybePrice.Float64/1000000, maybePrice.Valid)
}

func priceText(priceText string) string {
	if priceText == "" {
		return "Free"
	}
	return priceText
}

func extractPermissions(val gjson.Result) ([]Permission, error) {
	var permissions []Permission

	if !(val.IsArray() || val.Type == gjson.Null) {
		return nil, fmt.Errorf("expected an array")
	}

	for _, subVal := range val.Array() {
		if !val.IsArray() {
			return nil, fmt.Errorf("expected an array")
		}

		for _, rawPerm := range subVal.Array() {
			if !rawPerm.IsArray() {
				return nil, fmt.Errorf("expected an array")
			}

			rawPermArray := rawPerm.Array()

			if len(rawPermArray) == 0 {
				continue
			} else if len(rawPermArray) == 4 {
				extract := NewExtractor(rawPerm.String())
				group := extract.String("0")
				perms := extract.StringSlice("2.#.1")

				for _, perm := range perms {
					permissions = append(permissions, Permission{Group: group, Permission: perm})
				}
			} else if len(rawPermArray) == 2 {
				extract := NewExtractor(rawPerm.String())
				perm := extract.String("1")
				permissions = append(permissions, Permission{Group: "Other", Permission: perm})
			} else {
				return nil, fmt.Errorf("expected an array of length 2 or 4")
			}
		}
	}

	return permissions, nil
}

func ScrapeDetails(ctx context.Context, client *http.Client, appId string, country string, language string) (*Details, error) {
	requester := NewDetailsBatchRequester(appId)
	envelopes, err := sendRequests(ctx, client, country, language, []BatchRequester{requester})
	if err != nil {
		return nil, err
	}

	if len(envelopes) == 0 {
		return nil, fmt.Errorf("no envelope")
	}
	envelope := envelopes[0]

	details, err := requester.ParseEnvelope(envelope.Payload)
	if err != nil {
		return nil, err
	}

	return details.(*Details), nil
}
