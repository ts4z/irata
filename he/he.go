package he

import (
	"fmt"
	"log" // all kids love log
	"net/http"
)

// HTTPError probably represents the wrong abstraction.
type HTTPError struct {
	code int
	err  error
}

func HTTPCodedErrorf(code int, f string, more ...any) *HTTPError {
	return &HTTPError{
		code: code,
		err:  fmt.Errorf(f, more...),
	}
}

func New(code int, err error) *HTTPError {
	return &HTTPError{
		code: code,
		err:  err,
	}
}

func (e *HTTPError) Error() string {
	return e.err.Error()
}

// SendErrorToHTTPClient sends err as an HTTP error.  If it happens to be our
// special HTTPCodedError, we can include a better respone code; otherwise,
// client gets 500 and it's on us.
func SendErrorToHTTPClient(w http.ResponseWriter, while string, err error) {
	switch v := err.(type) {
	case *HTTPError:
		txt := fmt.Sprintf("can't %s: %v", while, v.err)
		log.Println(txt)
		http.Error(w, txt, v.code)

	default:
		http.Error(w, fmt.Sprintf("can't %s: %v", while, err), 500)
	}
}
