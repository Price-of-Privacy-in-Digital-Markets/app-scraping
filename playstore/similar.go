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
	AppId     string     `json:"app_id"`
	Title     string     `json:"title"`
	Developer string     `json:"developer"`
	Score     null.Float `json:"score"`
	ScoreText string     `json:"score_text"`
	Price     float64    `json:"price"`
	Currency  string     `json:"currency"`
}

type SimilarAppExtractError struct {
	Errors  []error
	Payload string
}

func (e *SimilarAppExtractError) Error() string {
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

func (br *similarBatchRequester) BatchRequest() batchRequest {
	return batchRequest{
		RpcId:   "ag2B9c",
		Payload: fmt.Sprintf(`[[null,["%s",7],null,[[3,[20]],true,null,[1,8]]],[true]]`, br.AppId),
	}
}

func (br *similarBatchRequester) ParseEnvelope(payload string) (interface{}, error) {
	result := gjson.Get(payload, "1.1.1.21.0")

	if result.Type == gjson.Null {
		// There are no similar apps
		return []SimilarApp{}, nil
	}

	extract := NewExtractor(result.Raw)

	appIds := extract.StringSlice("#.0.0")
	titles := extract.StringSlice("#.3")
	developers := extract.StringSlice("#.14")
	scores := extract.OptionalFloatSlice("#.4.1")
	scoreTexts := extract.StringSlice("#.4.0")
	prices := extract.FloatSlice("#.8.1.0.0")
	currencies := extract.StringSlice("#.8.1.0.1")

	if len(extract.Errors()) > 0 {
		return nil, &SimilarAppExtractError{Errors: extract.Errors(), Payload: payload}
	}

	n := len(appIds)
	similarApps := make([]SimilarApp, 0, n)

	for i := 0; i < n; i++ {
		similarApp := SimilarApp{
			AppId:     appIds[i],
			Title:     titles[i],
			Developer: developers[i],
			Score:     scores[i],
			ScoreText: scoreTexts[i],
			Price:     price(prices[i]),
			Currency:  currencies[i],
		}
		similarApps = append(similarApps, similarApp)
	}

	return similarApps, nil
}

func ScrapeSimilar(ctx context.Context, client *http.Client, appId string, country string, language string) ([]SimilarApp, error) {
	requester := &similarBatchRequester{AppId: appId}
	envelopes, err := sendRequests(ctx, client, country, language, []batchRequester{requester})
	if err != nil {
		return nil, err
	}

	if len(envelopes) == 0 {
		return nil, fmt.Errorf("no envelope")
	}
	envelope := envelopes[0]

	if len(envelope.Payload) == 0 {
		return nil, ErrAppNotFound
	}

	similar, err := requester.ParseEnvelope(envelope.Payload)
	if err != nil {
		return nil, err
	}

	return similar.([]SimilarApp), nil
}
