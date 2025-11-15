package middleware

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// CacheHeaderAdder wraps an http.Handler and adds cache-control headers.
// This is useful for static assets that can be cached by browsers.
type CacheHeaderAdder struct {
	enabled      bool
	maybe        func(r *http.Request) bool
	next         http.Handler
	maxAge       time.Duration
	immutable    bool
	cachePrivate bool
}

// CacheHeaderAdderConfig configures the caching behavior.
type CacheHeaderAdderConfig struct {
	// Add cache headers, but only if this returns true.
	Maybe func(r *http.Request) bool

	// Next is the handler to wrap.
	Next http.Handler

	// MaxAge is how long the content should be cached.
	// Common values: 1 hour, 1 day, 1 year for immutable assets.
	MaxAge time.Duration

	// Immutable indicates that the content will never change.
	// This is useful for versioned or content-addressed assets.
	Immutable bool

	// CachePrivate indicates that the content should only be cached
	// by the browser, not by shared caches (CDNs, proxies).
	CachePrivate bool
}

func envEnabled() bool {
	why := "there is a bug"
	enabled := true
	if env := os.Getenv("IRATA_ENABLE_CACHING"); env == "" {
		why = "not set, defaulting to enabled"
		enabled = true
	} else if v, err := strconv.Atoi(env); err != nil {
		why = fmt.Sprintf("can't parse IRATA_ENABLE_CACHING=%q: %v, caching disabled", env, err)
		enabled = false
	} else {
		enabled = v != 0
	}
	log.Printf("caching: %v (%s)", enabled, why)
	return enabled
}

// NewCacheHeaderAdder creates a new caching middleware.
func NewCacheHeaderAdder(config *CacheHeaderAdderConfig) *CacheHeaderAdder {
	return &CacheHeaderAdder{
		enabled:      envEnabled(),
		maybe:        config.Maybe,
		next:         config.Next,
		maxAge:       config.MaxAge,
		immutable:    config.Immutable,
		cachePrivate: config.CachePrivate,
	}
}

func (ch *CacheHeaderAdder) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if ch.maybe != nil && !ch.maybe(r) {
		ch.next.ServeHTTP(w, r)
		return
	}

	// Build the Cache-Control header value
	cacheControl := ""
	if ch.cachePrivate {
		cacheControl = "private"
	} else {
		cacheControl = "public"
	}

	maxAgeSeconds := int(ch.maxAge.Seconds())
	if maxAgeSeconds > 0 {
		if cacheControl != "" {
			cacheControl += ", "
		}
		cacheControl += fmt.Sprintf("max-age=%d", maxAgeSeconds)
	}

	if ch.immutable {
		if cacheControl != "" {
			cacheControl += ", "
		}
		cacheControl += "immutable"
	}

	if cacheControl != "" {
		w.Header().Set("Cache-Control", cacheControl)
	}

	ch.next.ServeHTTP(w, r)
}
