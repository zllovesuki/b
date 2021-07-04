package response

import (
	"encoding/json"
	"net/http"
)

type V1Response struct {
	Result   interface{} `json:"result"`
	Error    *string     `json:"error"`
	Messages []string    `json:"messages"`
}

func WriteError(w http.ResponseWriter, r *http.Request, e *Error) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(e.StatusCode)
	json.NewEncoder(w).Encode(V1Response{
		Result:   e.Result,
		Error:    &e.Message,
		Messages: e.Messages,
	})
}

func WriteResponse(w http.ResponseWriter, r *http.Request, response interface{}) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(V1Response{
		Result:   response,
		Error:    nil,
		Messages: []string{},
	})
}
