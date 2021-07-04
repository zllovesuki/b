package response

import "fmt"

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
	return makeError(500).
		WithMessage("An unexpected error has occured")
}

func ErrBadRequest() *Error {
	return makeError(400).
		WithMessage("Bad request")
}

func ErrNotFound() *Error {
	return makeError(404).
		WithMessage("Requested resources not found")
}

func ErrConflict() *Error {
	return makeError(409).
		WithMessage("Conflict")
}

func ErrInvalidJson() *Error {
	return ErrBadRequest().AddMessages("Invalid JSON body")
}
