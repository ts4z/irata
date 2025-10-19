package handlers

import (
	"io"
	"net/http"
)

func HandleRobotsTXT(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain")
	data := []string{
		"User-agent: *",
		"Allow: /",
		"Disallow: /*",
	}
	for _, line := range data {
		io.WriteString(w, line+"\r\n")
	}
}
