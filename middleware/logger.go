package middleware

import (
	"log"
	"net/http"
	"time"
)

type Clock interface {
	Now() time.Time
}

// RequestLogger is a middleware that logs the request.
//
// TODO: This is pretty underwheling, ain't it?
type RequestLogger struct {
	next  http.Handler
	clock Clock
}

func NewRequestLogger(next http.Handler, clock Clock) *RequestLogger {
	return &RequestLogger{next: next, clock: clock}
}

func remoteAddr(r *http.Request) string {
	if r.Header.Get("X-Forwarded-For") != "" {
		return r.Header.Get("X-Forwarded-For")
	}
	return r.RemoteAddr
}

func (rl *RequestLogger) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := rl.clock.Now()
	ww := &codeWatcher{w: w}
	rl.next.ServeHTTP(ww, r)
	code := ww.Code()
	duration := time.Since(start)
	log.Printf("[access log] %d %v %v (%v)", code, remoteAddr(r), r.URL.Path, duration)
}
