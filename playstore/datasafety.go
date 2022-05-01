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

// Based on documentation from
// https://support.google.com/googleplay/android-developer/answer/10787469
type DataSafety struct {
	// "Collect" means transmitting data from apps off a user’s device.
	Collection []DataCategory `json:"collection"`

	// "Sharing" refers to transferring user data collected from apps to a third party.
	Sharing []DataCategory `json:"sharing"`

	// Security practices
	SecurityPractices []string `json:"security_practices"`
}

type DataCategory struct {
	Name      string     `json:"category"`
	DataTypes []DataType `json:"data_types"`
}

func (dc *DataCategory) UnmarshalJSON(p []byte) error {
	var tmp []json.RawMessage
	if err := json.Unmarshal(p, &tmp); err != nil {
		return err
	}

	if len(tmp) != 5 {
		return fmt.Errorf("data category has invalid length")
	}

	// First unmarshall the name of the category

	var nameRaw []json.RawMessage
	if err := json.Unmarshal(tmp[0], &nameRaw); err != nil {
		return err
	}

	if len(nameRaw) != 3 {
		return fmt.Errorf("cannot unmarshal DataCategory: name should have length of 3")
	}

	if err := json.Unmarshal(nameRaw[1], &dc.Name); err != nil {
		return err
	}

	// Then unmarshall the data types

	if err := json.Unmarshal(tmp[4], &dc.DataTypes); err != nil {
		return err
	}

	return nil
}

type DataType struct {
	Name     string `json:"data_type"`
	Optional bool   `json:"optional"` // TODO: Or required??
	Purposes string `json:"purposes"`
}

func (dt *DataType) UnmarshalJSON(p []byte) error {
	var tmp []json.RawMessage
	if err := json.Unmarshal(p, &tmp); err != nil {
		return err
	}

	if len(tmp) != 3 {
		return fmt.Errorf("invalid length")
	}

	if err := json.Unmarshal(tmp[0], &dt.Name); err != nil {
		return err
	}

	if err := json.Unmarshal(tmp[1], &dt.Optional); err != nil {
		return err
	}

	if err := json.Unmarshal(tmp[2], &dt.Purposes); err != nil {
		return err
	}

	return nil
}

type envelope struct {
	RpcId   string
	Payload string
}

// TODO:
// Different envelopes I have seem are:
// wrb.fr - response
// di
// af.httprm
// https://kovatch.medium.com/deciphering-google-batchexecute-74991e4e446c
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
			if header != "wrb.fr" {
				return nil, fmt.Errorf("invalid header: %s", header)
			}

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

	return envelopes, nil
}

func ScrapeDataSafety(ctx context.Context, client *http.Client, appId string) (*DataSafety, error) {
	const dataSafetyUrl = "https://play.google.com/_/PlayStoreUi/data/batchexecute"

	form := url.Values{}
	dataSafetyBody := fmt.Sprintf(`[[["Ws7gDc","[null,null,[[1,69,70,96,100,138]],[[[true],null,[[[]]],null,null,null,null,[null,2],null,null,null,null,null,null,[1],null,null,null,null,null,null,null,[1]],[null,[[[]]]],[null,[[[]]],null,[true]],[null,[[[]]]],null,null,null,null,[[[[]]]],[[[[]]]]],null,[[\"%s\",7]]]",null,"1"]]]`, appId)
	form.Set("f.req", dataSafetyBody)

	req, err := http.NewRequestWithContext(ctx, "POST", dataSafetyUrl, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}

	// Build the URL
	params := req.URL.Query()
	params.Add("rpcids", "Ws7gDc")
	params.Add("source-path", "/store/apps/datasafety")
	params.Add("f.sid", "-2272275650025625973")
	params.Add("bl", "boq_playuiserver_20220427.02_p0")
	params.Add("hl", "en")
	params.Add("gl", "us")
	params.Add("authuser", "")
	params.Add("soc-app", "121")
	params.Add("soc-platform", "1")
	params.Add("soc-device", "1")
	params.Add("_reqid", "181072")
	req.URL.RawQuery = params.Encode()

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	// Even if the app is not found, the status is still 200 so don't check
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	envelopes, err := respToEnvelopes(body)
	if err != nil {
		return nil, err
	}

	// App does not exist
	if envelopes[0].Payload == "" {
		return nil, ErrAppNotFound
	}

	dataSafetyRaw := gjson.Get(envelopes[0].Payload, "1.2.137.4")
	if dataSafetyRaw.Value() == nil {
		// App does not have data safety section yet
		return nil, nil
	}

	dataSafety := &DataSafety{}

	if title := strings.TrimSpace(dataSafetyRaw.Get("0.1").String()); !(title == "Data shared with third parties" || title == "No data shared with third parties") {
		return nil, fmt.Errorf("unexpected data sharing title: %s", title)
	}
	if value := dataSafetyRaw.Get("0.0"); value.Exists() {
		// App shares data with third parties
		if err := json.Unmarshal([]byte(value.Raw), &dataSafety.Sharing); err != nil {
			return nil, err
		}
	}

	if title := dataSafetyRaw.Get("1.1").String(); !(title == "Data collected" || title == "No data collected") {
		return nil, fmt.Errorf("unexpected data collection title: %s", title)
	}
	if value := dataSafetyRaw.Get("1.0"); value.Exists() {
		// App collects data
		if err := json.Unmarshal([]byte(value.Raw), &dataSafety.Collection); err != nil {
			return nil, err
		}
	}

	// Security practices
	rawSecurityPractices := gjson.Get(envelopes[0].Payload, "1.2.137.9")

	if title := rawSecurityPractices.Get("1").String(); title != "Security practices" {
		return nil, fmt.Errorf("invalid security practices title: %s", title)
	}

	for _, practice := range rawSecurityPractices.Get("2.#.1").Array() {
		dataSafety.SecurityPractices = append(dataSafety.SecurityPractices, practice.String())
	}

	return dataSafety, nil
}
