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

func (rl *RequestLogger) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	start := rl.clock.Now()
	ww := &codeWatcher{w: w}
	rl.next.ServeHTTP(ww, r)
	code := ww.Code()
	duration := time.Since(start)
	log.Printf("[access] %d %v %v (%v)", code, r.RemoteAddr, r.URL.Path, duration)
}
