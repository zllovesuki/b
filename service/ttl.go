package service

import (
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
)

func ParseTTL(r *http.Request) int64 {
	ttlStr := chi.URLParam(r, "ttl")
	var ttl int64
	var err error
	if ttlStr != "" {
		ttl, err = strconv.ParseInt(ttlStr, 10, 64)
		if err != nil {
			ttl = 0
		}
	}
	return ttl
}
