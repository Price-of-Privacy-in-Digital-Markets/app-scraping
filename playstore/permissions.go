package playstore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Permission struct {
	Group      string `json:"group"`
	Permission string `json:"permission"`
}

type permissionsBatchRequester struct {
	AppId string
}

func (br *permissionsBatchRequester) BatchRequest() batchRequest {
	return batchRequest{
		RpcId:   "xdSrCf",
		Payload: fmt.Sprintf(`[[null,["%s",7],[]]]`, br.AppId),
	}
}

func (br *permissionsBatchRequester) ParseEnvelope(payload []byte) (permissions interface{}, err error) {
	// There are many type assertions so panic and recover instead of checking them all
	defer func() {
		r := recover()
		if r != nil {
			err = fmt.Errorf("panic when extracting permissions: %v", r)
		}
	}()

	if len(payload) == 0 {
		err = ErrAppNotFound
		return
	}

	// I think rawPermissions has length 3 but don't hardcode this just to be extra sure
	var rawPermissions [][][]interface{}
	err = json.Unmarshal([]byte(payload), &rawPermissions)
	if err != nil {
		return
	}

	var ps []Permission

	// Permissions in rawPermissions are either
	// - permissions with a permission group (e.g. Location or Microphone) - array of arrays of length 4
	// - "Other" permissions that have no group - array of arrays of length 2
	for _, permissionsList := range rawPermissions {
		for _, permissionItems := range permissionsList {
			switch len(permissionItems) {
			case 0:
				continue
			case 2:
				ps = append(ps, Permission{Group: "Other", Permission: permissionItems[1].(string)})
			case 4:
				group := permissionItems[0].(string)
				groupPerms := permissionItems[2].([]interface{})
				for _, perm := range groupPerms {
					perm := perm.([]interface{})
					ps = append(ps, Permission{Group: group, Permission: perm[1].(string)})
				}
			default:
				return nil, fmt.Errorf("extracting permissions: array of unexpected length: %v", permissionItems)
			}
		}
	}

	permissions = ps
	return
}

func ScrapePermissions(ctx context.Context, client *http.Client, appId string) ([]Permission, error) {
	requester := &permissionsBatchRequester{AppId: appId}
	envelopes, err := sendRequests(ctx, client, []batchRequester{requester})
	if err != nil {
		return nil, err
	}

	if len(envelopes) == 0 {
		return nil, fmt.Errorf("no envelope")
	}
	envelope := envelopes[0]

	permisions, err := requester.ParseEnvelope([]byte(envelope.Payload))
	if err != nil {
		return nil, err
	}

	return permisions.([]Permission), nil
}
