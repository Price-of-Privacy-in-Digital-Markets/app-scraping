package appstore

import (
	"errors"
	"strconv"
	"strings"
)

const fakeUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.141 Safari/537.36 Edg/87.0.664.75"

var ErrRateLimited = errors.New("Rate-limited")

func commaSeparatedAppIDs(appIds []AppId) string {
	var sb strings.Builder
	for i, id := range appIds {
		if i > 0 {
			sb.WriteString(",")
		}
		sb.WriteString(strconv.FormatInt(int64(id), 10))
	}
	return sb.String()
}
