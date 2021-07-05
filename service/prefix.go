package service

import (
	"fmt"
	"strings"
)

func Prefix(prefix, route string) string {
	prefix = strings.Trim(prefix, "-")
	return fmt.Sprintf("/%s-%s", prefix, route)
}

func Ret(baseURL, prefix, route string) string {
	baseURL = strings.Trim(baseURL, "/")
	baseURL = strings.Trim(baseURL, prefix)
	return fmt.Sprintf("%s%s", baseURL, Prefix(prefix, route))
}
