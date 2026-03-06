package server

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/tagus/mango"
)

type CoreHandler struct {
	params Params
}

func NewCoreHandler(params Params) *CoreHandler {
	return &CoreHandler{params: params}
}

func (h *CoreHandler) Health(w http.ResponseWriter, _ *http.Request) error {
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"status": "healthy", "app": h.params.Config.App.Name, "version": h.params.Version})
	return nil
}

func (h *CoreHandler) ServiceWorker(w http.ResponseWriter, _ *http.Request) error {
	buf, err := os.ReadFile(filepath.Join(h.params.StaticDir, "sw.js"))
	if err != nil {
		return mango.NotFoundError("service worker not found")
	}
	content := strings.ReplaceAll(string(buf), "__APP_VERSION__", h.params.Version)
	w.Header().Set("Content-Type", "application/javascript")
	_, _ = w.Write([]byte(content))
	return nil
}

func (h *CoreHandler) LoginPage(w http.ResponseWriter, r *http.Request) error {
	if !h.params.Auth.Enabled() {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return nil
	}
	if cookie, _ := r.Cookie("nd_auth"); cookie != nil && h.params.Auth.IsAuthenticated(cookie.Value) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return nil
	}
	buf, err := os.ReadFile(filepath.Join(h.params.StaticDir, "login.html"))
	if err != nil {
		return err
	}
	content := strings.ReplaceAll(string(buf), "NoteDiscovery", h.params.Config.App.Name)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(content))
	return nil
}

func (h *CoreHandler) Login(w http.ResponseWriter, r *http.Request) error {
	if !h.params.Auth.Enabled() {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return nil
	}
	if err := r.ParseForm(); err != nil {
		return mango.BadRequestErrorWithCause("invalid form", err)
	}
	password := r.Form.Get("password")
	if !h.params.Auth.VerifyPassword(password) {
		http.Redirect(w, r, "/login?error=incorrect_password", http.StatusSeeOther)
		return nil
	}
	http.SetCookie(w, &http.Cookie{Name: "nd_auth", Value: h.params.Auth.NewCookieValue(), Path: "/", MaxAge: h.params.Auth.MaxAge(), HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
}

func (h *CoreHandler) Logout(w http.ResponseWriter, r *http.Request) error {
	http.SetCookie(w, &http.Cookie{Name: "nd_auth", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
	return nil
}

func (h *CoreHandler) GetConfig(w http.ResponseWriter, _ *http.Request) error {
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{
		"name":           h.params.Config.App.Name,
		"version":        h.params.Version,
		"searchEnabled":  h.params.Config.Search.Enabled,
		"demoMode":       strings.EqualFold(os.Getenv("DEMO_MODE"), "true"),
		"alreadyDonated": strings.EqualFold(os.Getenv("ALREADY_DONATED"), "true"),
		"authentication": map[string]any{"enabled": h.params.Config.Authentication.Enabled},
	})
	return nil
}

func (h *CoreHandler) CatchAll(w http.ResponseWriter, _ *http.Request) error {
	buf, err := os.ReadFile(filepath.Join(h.params.StaticDir, "index.html"))
	if err != nil {
		return err
	}
	content := strings.ReplaceAll(string(buf), "<title>NoteDiscovery</title>", "<title>"+h.params.Config.App.Name+"</title>")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(content))
	return nil
}

func (h *CoreHandler) corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origins := h.params.Config.Server.AllowedOrigins
		origin := "*"
		if len(origins) == 1 {
			origin = origins[0]
		}
		if origin == "*" {
			w.Header().Set("Access-Control-Allow-Origin", "*")
		} else {
			w.Header().Set("Access-Control-Allow-Origin", origin)
		}
		w.Header().Set("Access-Control-Allow-Credentials", "true")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		w.Header().Set("Access-Control-Allow-Methods", "GET,POST,DELETE,OPTIONS")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *CoreHandler) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !h.params.Auth.Enabled() {
			next.ServeHTTP(w, r)
			return
		}
		cookie, _ := r.Cookie("nd_auth")
		if cookie == nil || !h.params.Auth.IsAuthenticated(cookie.Value) {
			if strings.HasPrefix(r.URL.Path, "/api/") {
				mango.WriteJSONResponse(w, http.StatusUnauthorized, map[string]any{"detail": "Not authenticated"})
				return
			}
			http.Redirect(w, r, "/login", http.StatusSeeOther)
			return
		}
		next.ServeHTTP(w, r)
	})
}
