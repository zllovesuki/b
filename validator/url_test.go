package validator

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// https://stackoverflow.com/questions/31480710/validate-url-with-standard-package-in-go
// Adopted from: https://stackoverflow.com/questions/31480710/validate-url-with-standard-package-in-go
func TestURLValidation(t *testing.T) {
	t.Run("bad url should return false", func(t *testing.T) {
		badURLs := []string{
			"https",
			"https://",
			"",
			"http://www",
			"/testing-path",
			"testing-path",
			"alskjff#?asf//dfas",
		}
		for _, u := range badURLs {
			require.False(t, URL(u), "\"%s\" should not be valid", u)
		}
	})
	t.Run("good url should return true", func(t *testing.T) {
		goodURLs := []string{
			"https://google.com",
			"https://google.com/hello",
			"https://google.com:44/hello",
		}
		for _, u := range goodURLs {
			require.True(t, URL(u), "\"%s\" should be valid", u)
		}
	})
}
