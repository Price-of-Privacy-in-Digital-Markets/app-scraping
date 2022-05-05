package database

import (
	"regexp"
	"strings"
)

var validAppIdRegex = regexp.MustCompile(`^([a-z][a-z0-9_]*)(\.[a-z][a-z0-9_]*)+$`)

func validAppId(appId string) bool {
	return validAppIdRegex.MatchString(strings.ToLower(appId))
}
