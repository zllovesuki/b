package service

import (
	"net/http"

	"github.com/zllovesuki/b/response"

	"go.uber.org/zap"
)

// Recovery will catch panic and send to logger, then respond with 500
func Recovery(logger *zap.Logger) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				err := recover()
				if err != nil && err != http.ErrAbortHandler {
					logger.Error("Handler panic",
						zap.Any("Exception", err),
					)
					response.WriteError(w, r, response.ErrUnexpected().AddMessages("Server has encountered an unrecoverable error"))
				}
			}()

			next.ServeHTTP(w, r)
		})
	}
}
