package playstore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
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

type dataSafetyRequester struct {
	AppId string
}

func (br *dataSafetyRequester) BatchRequest() batchRequest {
	return batchRequest{
		RpcId:   "Ws7gDc",
		Payload: fmt.Sprintf(`[null,null,[[1,69,70,96,100,138]],[[[true],null,[[[]]],null,null,null,null,[null,2],null,null,null,null,null,null,[1],null,null,null,null,null,null,null,[1]],[null,[[[]]]],[null,[[[]]],null,[true]],[null,[[[]]]],null,null,null,null,[[[[]]]],[[[[]]]]],null,[["%s",7]]]`, br.AppId),
	}
}

func (br *dataSafetyRequester) ParseEnvelope(payload string) (interface{}, error) {
	if len(payload) == 0 {
		return nil, ErrAppNotFound
	}

	dataSafetyRaw := gjson.Get(payload, "1.2.137.4")
	if dataSafetyRaw.Value() == nil {
		// App does not have data safety section yet
		return nil, nil
	}

	var dataSafety DataSafety

	if title := strings.TrimSpace(dataSafetyRaw.Get("0.1").String()); !(title == "Data shared" || title == "No data shared with third parties") {
		return nil, fmt.Errorf("unexpected data sharing title: %s", title)
	}
	if value := dataSafetyRaw.Get("0.0"); value.Exists() {
		// App shares data with third parties
		if err := json.Unmarshal([]byte(value.Raw), &dataSafety.Sharing); err != nil {
			return nil, err
		}
		if dataSafety.Sharing == nil {
			dataSafety.Sharing = []DataCategory{}
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
		if dataSafety.Collection == nil {
			dataSafety.Collection = []DataCategory{}
		}
	}

	// Security practices
	rawSecurityPractices := gjson.Get(payload, "1.2.137.9")
	if rawSecurityPractices.Exists() {
		if title := rawSecurityPractices.Get("1").String(); title != "Security practices" {
			return nil, fmt.Errorf("invalid security practices title: %s", title)
		}

		for _, practice := range rawSecurityPractices.Get("2.#.1").Array() {
			dataSafety.SecurityPractices = append(dataSafety.SecurityPractices, practice.String())
		}
	} else {
		dataSafety.SecurityPractices = []string{}
	}

	return &dataSafety, nil
}

func ScrapeDataSafety(ctx context.Context, client *http.Client, appId string) (*DataSafety, error) {
	requester := &dataSafetyRequester{AppId: appId}

	envelopes, err := sendRequests(ctx, client, "us", "en", []batchRequester{requester})
	if err != nil {
		return nil, err
	}

	if len(envelopes) == 0 {
		return nil, fmt.Errorf("no envelope")
	}

	envelope := envelopes[0]

	ds, err := requester.ParseEnvelope(envelope.Payload)
	if err != nil {
		return nil, err
	}

	if ds == nil {
		return nil, nil
	}
	return ds.(*DataSafety), nil
}
