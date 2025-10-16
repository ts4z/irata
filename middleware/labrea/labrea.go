// package labrea provides a middleware that provides a tarpit.
//
// This could probably generate better trash.
package labrea

import (
	"encoding/hex"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"sync"
	"time"
)

type Handler struct {
	paths map[string]struct{}
	rand  *rand.Rand

	mu      sync.Mutex
	ipCount map[string]int

	next http.Handler
}

var _ http.Handler = &Handler{}

var defaultPaths = []string{
	"/.env",
	"/.git/",
	"/web/wp-includes/wlwmanifest.xml",
	"/web/wp-admin/",
	"/feed/",
	"/xmlrpc.php",
	"/wp-login.php",
	"/wp-admin/",
	"/admin/",
	"/login/",
	"/user/login/",
	"/cms/",
	"/phpmyadmin/",
	"/pma/",
	"/myadmin/",
	"/mysql/",
	"/dbadmin/",
	"/sqladmin/",
	"/db/",
	"/database/",
	"/.git",
	"/.htaccess",
	"/.htpasswd",
	"/config.php",
	"/config.inc.php",
	"/install.php",
	"/setup.php",
	"/readme.html",
	"/license.txt",
	"/server-status",
	"/web/wp-login.php",
	"/web/wp-admin/",
	"/web/wp-includes/",
	"/wordpress/wp-login.php",
	"/wordpress/wp-admin/",
	"/blog/wp-login.php",
	"/blog/wp-admin/",
	"/2021/wp-includes/wlmanifest.xml",
	"/2020/wp-includes/wlmanifest.xml",
	"/2019/wp-includes/wlmanifest.xml",
	"/2018/wp-includes/wlmanifest.xml",
	"/2017/wp-includes/wlmanifest.xml",
	"/2016/wp-includes/wlmanifest.xml",
	"/shop/wp-includes/wlmanifest.xml",
	"/wp/wp-includes/wlmanifest.xml",
	"/wp1/wp-includes/wlmanifest.xml",
	"/new/wp-includes/wlmanifest.xml",
}

func New(next http.Handler) *Handler {
	pathMap := make(map[string]struct{}, len(defaultPaths))
	for _, p := range defaultPaths {
		pathMap[p] = struct{}{}
	}
	return &Handler{
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
	w.Header().Set("Server", "Apache/2.4.41 (Ubuntu)")
	w.WriteHeader(http.StatusOK)

	// Send a plausible-looking but garbage HTML response
	w.Write([]byte(`<!DOCTYPE html>
<html>
<head>
    <title>Loading...</title>
    <meta charset="UTF-8">
</head>
<body>
`))

	h.flush(w)

	// Send garbage data slowly, chunk by chunk
	for range 20 + h.rand.Intn(50) {
		time.Sleep(h.randomDelay(100*time.Millisecond, 2*time.Second))
		w.Write([]byte(h.generateGarbageHTML()))
		h.flush(w)
	}

	// Close the HTML properly to look legitimate
	w.Write([]byte(`
</body>
</html>
`))
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

// generateGarbageHTML creates plausible-looking but useless HTML content
func (h *Handler) generateGarbageHTML() string {
	// Generate random hex strings to pad the response
	randomBytes := make([]byte, 128)
	h.rand.Read(randomBytes)
	hexData := hex.EncodeToString(randomBytes)

	// Return as HTML comments or hidden divs
	templates := []func() string{
		func() string { return fmt.Sprintf("<!-- %s -->\n", hexData) },
		func() string { return fmt.Sprintf("<div style='display:none'>%s</div>\n", hexData) },
		func() string { return fmt.Sprintf("<script>/* %s */</script>\n", hexData) },
		func() string { return fmt.Sprintf("    <p style='opacity:0'>%s</p>\n", hexData) },
	}

	// Pick a random template
	n := h.rand.Intn(len(templates))
	return templates[n]()
}

func (h *Handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if _, ok := h.paths[r.URL.Path]; ok {
		log.Printf("tarpitting request for %s from %s", r.URL.Path, r.RemoteAddr)
		h.Mishandle(w, r)
		return
	}

	h.next.ServeHTTP(w, r)
}
