package handlers

import (
	"context"
	"io"
	"net/http"
)

func HandleRobotsTXT(_ context.Context, w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	data := []string{
		"User-agent: *",
		"Allow: /",
		"Disallow: /t/*",
		"Disallow: /login",
		"Disallow: /manage/*",
		"Disallow: /create/*",
		"Disallow: /sqladmin",
		"Disallow: /wp-admin/*",
		"Disallow: /xmlrpc.php",
		"Disallow: /wordpress/*",
	}
	for _, line := range data {
		io.WriteString(w, line)
		io.WriteString(w, "\r\n")
	}
}
