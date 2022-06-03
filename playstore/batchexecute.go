package playstore

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/tidwall/gjson"
)

var ErrRateLimited error = errors.New("google detected unusual traffic")

type envelope struct {
	RpcId   string
	Payload string
	Number  int
}

type BatchRequester interface {
	BatchRequest() batchRequest
	ParseEnvelope(string) (interface{}, error)
}

type batchRequest struct {
	RpcId   string
	Payload string
}

// See https://kovatch.medium.com/deciphering-google-batchexecute-74991e4e446c for more
// information about Google's RPC. We are interested in the wrb.fr response.
func respToEnvelopes(body []byte) ([]envelope, error) {
	if bytes.HasPrefix(body, []byte("<!DOCTYPE html")) {
		// Google is not happy with us :(
		return nil, ErrRateLimited
	}

	if !bytes.HasPrefix(body, []byte(")]}'\n\n")) {
		fmt.Println(string(body))
		return nil, fmt.Errorf("invalid response")
	}

	body = bytes.TrimPrefix(body, []byte(")]}'\n\n"))

	var wrapper [][]json.RawMessage
	if err := json.Unmarshal(body, &wrapper); err != nil {
		return nil, err
	}

	var envelopes []envelope
	for _, rawEnvelope := range wrapper {
		if len(rawEnvelope) == 7 {
			var header string
			var envelope envelope

			if err := json.Unmarshal(rawEnvelope[0], &header); err != nil {
				return nil, err
			}
			if header == "wrb.fr" {
				if err := json.Unmarshal(rawEnvelope[1], &envelope.RpcId); err != nil {
					return nil, err
				}

				if err := json.Unmarshal(rawEnvelope[2], &envelope.Payload); err != nil {
					return nil, err
				}
				if envelope.Payload != "" && !gjson.Valid(envelope.Payload) {
					return nil, fmt.Errorf("envelope has invalid JSON payload")
				}

				var number string
				if err := json.Unmarshal(rawEnvelope[6], &number); err != nil {
					return nil, err
				}
				if n, err := strconv.Atoi(number); err != nil {
					return nil, err
				} else {
					envelope.Number = n
				}

				envelopes = append(envelopes, envelope)
			}
		}
	}

	return envelopes, nil
}

func sendRequests(ctx context.Context, client *http.Client, country string, language string, requesters []BatchRequester) ([]envelope, error) {
	const batchExecuteUrl = "https://play.google.com/_/PlayStoreUi/data/batchexecute"

	// Make the body of the request
	rpcids := make([]string, 0, len(requesters))
	fReq := make([][]*string, 0, len(requesters))
	for i, requester := range requesters {
		br := requester.BatchRequest()
		num := fmt.Sprintf("%d", i)
		fReq = append(fReq, []*string{&br.RpcId, &br.Payload, nil, &num})
		rpcids = append(rpcids, br.RpcId)
	}

	fReqJson, err := json.Marshal([][][]*string{fReq})
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("f.req", string(fReqJson))

	req, err := http.NewRequestWithContext(ctx, "POST", batchExecuteUrl, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	// Build the URL
	params := req.URL.Query()
	params.Add("rpcids", strings.Join(rpcids, ","))
	params.Add("f.sid", "-2272275650025625973")
	params.Add("hl", language)
	params.Add("gl", country)
	params.Add("authuser", "")
	params.Add("_reqid", "181072")
	req.URL.RawQuery = params.Encode()

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	envelopes, err := respToEnvelopes(body)
	if err != nil {
		return nil, err
	}

	return envelopes, nil
}

func SendBatchedRequests(ctx context.Context, client *http.Client, country string, language string, requesters []BatchRequester) ([]interface{}, error) {
	envelopes, err := sendRequests(ctx, client, country, language, requesters)
	if err != nil {
		return nil, err
	}

	output := make([]interface{}, len(requesters))

	for _, envelope := range envelopes {
		output[envelope.Number], err = requesters[envelope.Number].ParseEnvelope(envelope.Payload)
		if err != nil {
			return nil, err
		}
	}

	return output, nil
}
