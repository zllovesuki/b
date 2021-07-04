package response

import (
	"fmt"
	"net/http"
)

type Error struct {
	StatusCode int
	Message    string
	Messages   []string
	Result     interface{}
}

func (e *Error) Error() string {
	return fmt.Sprintf("HTTP %d: %s", e.StatusCode, e.Message)
}

func (e *Error) WithMessage(msg string) *Error {
	e.Message = msg
	return e
}

func (e *Error) AddMessages(msgs ...string) *Error {
	e.Messages = append(e.Messages, msgs...)
	return e
}

func (e *Error) WithResult(result interface{}) *Error {
	e.Result = result
	return e
}

func makeError(status int) *Error {
	return &Error{
		StatusCode: status,
		Messages:   make([]string, 0),
		Result:     []string{},
	}
}

// -----------------------------------------------

func ErrUnexpected() *Error {
	return makeError(http.StatusInternalServerError).
		WithMessage("An unexpected error has occured")
}

func ErrBadRequest() *Error {
	return makeError(http.StatusBadRequest).
		WithMessage("Bad request")
}

func ErrNotFound() *Error {
	return makeError(http.StatusNotFound).
		WithMessage("Requested resources not found")
}

func ErrConflict() *Error {
	return makeError(http.StatusConflict).
		WithMessage("Conflict")
}

func ErrInvalidJson() *Error {
	return ErrBadRequest().AddMessages("Invalid JSON body")
}

func ErrorMethodNotAllowed() *Error {
	return makeError(http.StatusMethodNotAllowed).AddMessages("Method not allowed")
}
