package playstore

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Permission struct {
	Group      string
	Permission string
}

func ScrapePermissions(ctx context.Context, client *http.Client, appId string) (permissions []Permission, err error) {
	// There are many type assertions so panic and recover instead of checking them all
	defer func() {
		r := recover()
		if r != nil {
			err = fmt.Errorf("panic when extracting permissions: %w", r)
		}
	}()

	permissionUrl := "https://play.google.com/_/PlayStoreUi/data/batchexecute?rpcids=qnKhOb&f.sid=-697906427155521722&bl=boq_playuiserver_20190903.08_p0&hl=en&authuser&soc-app=121&soc-platform=1&soc-device=1&_reqid=1065213"

	form := url.Values{}
	form.Set("f.req", fmt.Sprintf(`[[["xdSrCf","[[null,[\"%s\",7],[]]]",null,"1"]]]`, appId))

	req, err := http.NewRequestWithContext(ctx, "POST", permissionUrl, strings.NewReader(form.Encode()))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded;charset=UTF-8")

	resp, err := client.Do(req)
	if err != nil {
		return
	}

	// Even if the app is not found, the status is still 200 so don't check...
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return
	}

	var input interface{}
	if err = json.Unmarshal(body[5:], &input); err != nil {
		return
	}

	data, ok := pluckPanic(input, 0, 2).(string)
	if !ok {
		err = &AppNotFoundError{appId}
		return
	}

	// I think rawPermissions has length 3 but don't hardcode this just to be extra sure
	var rawPermissions [][][]interface{}
	err = json.Unmarshal([]byte(data), &rawPermissions)
	if err != nil {
		return
	}

	// Permissions in rawPermissions are either
	// - permissions with a permission group (e.g. Location or Microphone) - array of arrays of length 4
	// - "Other" permissions that have no group - array of arrays of length 2
	for _, permissionsList := range rawPermissions {
		for _, permissionItems := range permissionsList {
			switch len(permissionItems) {
			case 0:
				continue
			case 2:
				permissions = append(permissions, Permission{Group: "Other", Permission: permissionItems[1].(string)})
			case 4:
				group := permissionItems[0].(string)
				groupPerms := permissionItems[2].([]interface{})
				for _, perm := range groupPerms {
					perm := perm.([]interface{})
					permissions = append(permissions, Permission{Group: group, Permission: perm[1].(string)})
				}
			default:
				err = fmt.Errorf("extracting permissions: array of unexpected length: %v", permissionItems)
				return
			}
		}
	}

	return
}
