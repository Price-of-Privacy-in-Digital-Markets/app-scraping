package playstore

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/tidwall/gjson"
	"gopkg.in/guregu/null.v4"
)

type SimilarApp struct {
	AppId     string      `json:"app_id"`
	Title     string      `json:"title"`
	Developer string      `json:"developer"`
	Score     null.Float  `json:"score"`
	ScoreText null.String `json:"score_text"`
	Price     null.Float  `json:"price"`
	Currency  null.String `json:"currency"`
}

type SimilarAppsExtractError struct {
	Errors  []error
	Payload string
}

func (e *SimilarAppsExtractError) Error() string {
	sb := strings.Builder{}

	sb.WriteString("Error extracting similar apps:\n")
	for _, err := range e.Errors {
		sb.WriteString(fmt.Sprintf("\t- %s\n", err.Error()))
	}

	return sb.String()
}

type similarBatchRequester struct {
	AppId string
}

func NewSimilarBatchRequester(appId string) *similarBatchRequester {
	return &similarBatchRequester{AppId: appId}
}

func (br *similarBatchRequester) BatchRequest() batchRequest {
	return batchRequest{
		RpcId:   "ag2B9c",
		Payload: fmt.Sprintf(`[[null,["%s",7],null,[[3,[20]],true,null,[1,8]]],[true]]`, br.AppId),
	}
}

func (br *similarBatchRequester) ParseEnvelope(payload string) (interface{}, error) {
	if payload == "" {
		return nil, ErrAppNotFound
	}

	result := gjson.Get(payload, "1.1.1.21.0")

	if result.Type == gjson.Null {
		// There are no similar apps
		return []SimilarApp{}, nil
	}

	if !result.IsArray() {
		return nil, fmt.Errorf("wrong type: not array")
	}

	rawApps := result.Array()
	similarApps := make([]SimilarApp, 0, len(rawApps))

	for _, rawApp := range rawApps {
		extract := NewExtractor(rawApp.Raw)

		similarApp := SimilarApp{
			AppId:     extract.String("0.0"),
			Title:     extract.String("3"),
			Developer: extract.String("14"),
			Score:     extract.OptionalFloat("4.1"),
			ScoreText: extract.OptionalString("4.0"),
			Price:     maybePrice(extract.OptionalFloat("8.1.0.0")),
			Currency:  extract.OptionalString("8.1.0.1"),
		}

		if len(extract.Errors()) > 0 {
			return nil, &SimilarAppsExtractError{Errors: extract.Errors(), Payload: payload}
		}

		similarApps = append(similarApps, similarApp)
	}

	return similarApps, nil
}

func ScrapeSimilar(ctx context.Context, client *http.Client, appId string, country string, language string) ([]SimilarApp, error) {
	requester := NewSimilarBatchRequester(appId)
	envelopes, err := sendRequests(ctx, client, country, language, []BatchRequester{requester})
	if err != nil {
		return nil, err
	}

	if len(envelopes) == 0 {
		return nil, fmt.Errorf("no envelope")
	}
	envelope := envelopes[0]

	similar, err := requester.ParseEnvelope(envelope.Payload)
	if err != nil {
		return nil, err
	}

	return similar.([]SimilarApp), nil
}
