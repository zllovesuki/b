package validator

import (
	"net/url"
	"strings"
)

// URL checks if the provided URL is a valid URL
// Adopted from: https://stackoverflow.com/questions/31480710/validate-url-with-standard-package-in-go
func URL(str string) bool {
	u, err := url.Parse(str)
	if err != nil {
		return false
	}

	hasHost := u.Scheme != "" && u.Hostname() != ""
	if !hasHost {
		return false
	}

	if !strings.Contains(u.Hostname(), ".") {
		return false
	}

	return true
}
