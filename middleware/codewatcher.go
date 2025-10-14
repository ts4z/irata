package middleware

import (
	"net/http"
)

var _ http.ResponseWriter = &codeWatcher{}

// codeWatcher is a http.ResponseWriter that captures the status code for logging.
type codeWatcher struct {
	code *int
	w    http.ResponseWriter
}

func (cw *codeWatcher) Header() http.Header {
	return cw.w.Header()
}

func (cw *codeWatcher) Write(b []byte) (int, error) {
	return cw.w.Write(b)
}

func (cw *codeWatcher) WriteHeader(statusCode int) {
	cw.code = &statusCode
	cw.w.WriteHeader(statusCode)
}

func (cw *codeWatcher) Code() int {
	if cw.code != nil {
		return *cw.code
	} else {
		return 200
	}
}
