package utils

import (
	"regexp"
)

func SanitizeString(s string) string {
	return replace(s, `\W`)
}

func SanitizeUrl(s string) string {
	return replace(s, `[^a-zA-Z0-9-_/:.,?&@=#%]`)
}

func replace(s string, r string) string {
	re := regexp.MustCompile(r)
	return string(re.ReplaceAll([]byte(s), []byte("")))
}
