package httputil

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func PathParam(r *http.Request, named string) string {
	value := strings.TrimPrefix(chi.URLParam(r, named), "/")
	if value != "" {
		return value
	}
	return strings.TrimPrefix(chi.URLParam(r, "*"), "/")
}

func SchemeFor(r *http.Request) string {
	if r.Header.Get("X-Forwarded-Proto") == "https" || r.TLS != nil {
		return "https"
	}
	return "http"
}
