package playstore

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/tidwall/gjson"
)

type envelope struct {
	RpcId   string
	Payload string
}

type batchRequester interface {
	BatchRequest() batchRequest
	ParseEnvelope([]byte) (interface{}, error)
}

type batchRequest struct {
	RpcId   string
	Payload string
}

// See https://kovatch.medium.com/deciphering-google-batchexecute-74991e4e446c for more
// information about Google's RPC. We are interested in the wrb.fr response.
func respToEnvelopes(body []byte) ([]envelope, error) {
	if !bytes.HasPrefix(body, []byte(")]}'\n\n")) {
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

				envelopes = append(envelopes, envelope)
			}
		}
	}

	return envelopes, nil
}

func sendRequests(ctx context.Context, client *http.Client, requesters []batchRequester) ([]envelope, error) {
	const dataSafetyUrl = "https://play.google.com/_/PlayStoreUi/data/batchexecute"

	// Make the body of the request
	rpcids := make([]string, 0, len(requesters))
	fReq := make([][]*string, 0, len(requesters))
	for i, requester := range requesters {
		br := requester.BatchRequest()
		num := fmt.Sprintf("%d", i+1)
		fReq = append(fReq, []*string{&br.RpcId, &br.Payload, nil, &num})
		rpcids = append(rpcids, br.RpcId)
	}

	fReqJson, err := json.Marshal([][][]*string{fReq})
	if err != nil {
		return nil, err
	}

	form := url.Values{}
	form.Set("f.req", string(fReqJson))

	req, err := http.NewRequestWithContext(ctx, "POST", dataSafetyUrl, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	// Build the URL
	params := req.URL.Query()
	params.Add("rpcids", strings.Join(rpcids, ","))
	params.Add("f.sid", "-2272275650025625973")
	params.Add("hl", "en")
	params.Add("gl", "us")
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
