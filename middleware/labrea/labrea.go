// package labrea provides a middleware that provides a tarpit.
//
// This could do a lot of things that it doesn't, but it doesn't mess up
// the logs.
package labrea

import (
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Clock interface {
	Now() time.Time
}

type Handler struct {
	clock Clock
	rand  *rand.Rand
	next  http.Handler

	paths map[string]struct{}

	mu      sync.Mutex
	ipCount map[string]int
}

var _ http.Handler = &Handler{}

var defaultPaths = []string{
	".env",
	".git",
	".htaccess",
	".htpasswd",
	"admin",
	"blog/wp-admin",
	"blog/wp-login.php",
	"cms",
	"config.inc.php",
	"config.php",
	"database",
	"db",
	"dbadmin",
	"feed",
	"install.php",
	"license.txt",
	"myadmin",
	"mysql",
	"phpmyadmin",
	"pma",
	"readme.html",
	"server-status",
	"setup.php",
	"sqladmin",
	"user/login",
	"web/wp-admin",
	"web/wp-includes",
	"web/wp-login.php",
	"wordpress/wp-admin",
	"wordpress/wp-login.php",
	"wp-admin",
	"wp-admin/setup-config.php",
	"wp-includes/wlmanifest.xml",
	"wp-includes/wlwmanifest.xml",
	"wp-login.php",
	"xmlrpc.php",
}

func New(clock Clock, next http.Handler) *Handler {
	pathMap := make(map[string]struct{}, len(defaultPaths))
	for _, p := range defaultPaths {
		pathMap[p] = struct{}{}
	}
	return &Handler{
		clock:   clock,
		mu:      sync.Mutex{},
		ipCount: map[string]int{},
		next:    next,
		rand:    rand.New(rand.NewSource(time.Now().UnixNano())),
		paths:   pathMap,
	}
}

func (h *Handler) countIP(ip string) int {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.ipCount[ip]++
	if len(h.ipCount) > 1000 {
		// Reset if too many entries
		h.ipCount = map[string]int{}
	}

	return h.ipCount[ip]
}

func (h *Handler) flush(w http.ResponseWriter) {
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
}

func (h *Handler) Mishandle(w http.ResponseWriter, r *http.Request) {
	minimum := time.Duration(11*h.countIP(r.RemoteAddr)) * time.Millisecond
	initialDelay := h.randomDelay(minimum, 3*time.Second)
	time.Sleep(initialDelay)

	// Set headers to make it look legitimate
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Server", "Apache")
	w.WriteHeader(http.StatusNotFound)

	payload := []byte(`
<!DOCTYPE HTML PUBLIC "-//IETF//DTD HTML 2.0//EN">
<html><head>
<title>404 Not Found</title>
</head><body>
<h1>Not Found</h1>
<p>The requested URL was not found on this server.</p>
<p>Additionally, a 404 Not Found
error was encountered while trying to use an ErrorDocument to handle the request.</p>
</body></html>
`)

	for pos := 0; pos < len(payload); {
		remainder := len(payload) - pos
		amt := min(10+rand.Intn(10), remainder)
		w.Write(payload[pos : pos+amt])
		pos += amt
		h.randomDelay(100*time.Millisecond, 300*time.Millisecond)
		h.flush(w)
	}
}

// randomDelay returns a random duration between min and max milliseconds.
// The arguments can actually appear in either order.
func (h *Handler) randomDelay(min, max time.Duration) time.Duration {
	if min > max {
		return max
	}
	delay := h.rand.Intn(int(max - min))
	return min + time.Duration(delay)*time.Nanosecond
}

func last2(path string) string {
	parts := strings.Split(path, "/")
	for len(parts) > 1 && parts[0] == "" {
		parts = parts[1:]
	}
	for len(parts) >= 1 && parts[len(parts)-1] == "" {
		parts = parts[:len(parts)-1]
	}
	if len(parts) == 0 {
		return path
	}
	first := max(0, len(parts)-2)
	last := len(parts)
	s := strings.Join(parts[first:last], "/")
	return s
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// start := h.clock.Now()
	path := last2(r.URL.Path)
	if _, ok := h.paths[path]; ok {
		h.Mishandle(w, r)
		// duration := h.clock.Now().Sub(start)
		// log.Printf("[tarpit] 404 %v %v (%v)", r.RemoteAddr, r.URL.Path, duration)
		return
	}

	h.next.ServeHTTP(w, r)
}
