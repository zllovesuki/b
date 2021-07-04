package service

import (
	"net/http"

	"github.com/go-chi/chi"
	"github.com/zllovesuki/b/response"
)

func NotImplemented() http.Handler {
	r := chi.NewRouter()

	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		response.WriteError(w, r, response.ErrorMethodNotAllowed())
	})

	return r
}
