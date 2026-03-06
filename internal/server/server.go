package server

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"notediscovery/internal/auth"
	"notediscovery/internal/config"
	"notediscovery/internal/graph"
	"notediscovery/internal/locales"
	"notediscovery/internal/notes"
	"notediscovery/internal/plugins"
	"notediscovery/internal/share"
	"notediscovery/internal/themes"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/tagus/mango"
)

type Server struct {
	srv      *http.Server
	port     int
	handlers *Handlers
}

type Params struct {
	Config     *config.Config
	Version    string
	StaticDir  string
	ThemesDir  string
	LocalesDir string
	Notes      *notes.Service
	Themes     *themes.Service
	Locales    *locales.Service
	Plugins    *plugins.Service
	Graph      *graph.Service
	Share      *share.Service
	Auth       *auth.Service
}

func New(params Params) *Server {
	h := &Handlers{params: params}
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(middleware.StripSlashes)
	r.Use(h.corsMiddleware)

	r.Get("/health", mango.WrapErrorHandler(h.Health))
	r.Get("/sw.js", mango.WrapErrorHandler(h.ServiceWorker))
	r.Get("/login", mango.WrapErrorHandler(h.LoginPage))
	r.Post("/login", mango.WrapErrorHandler(h.Login))
	r.Get("/logout", mango.WrapErrorHandler(h.Logout))

	// Public routes needed for login page/app bootstrap.
	r.Get("/api/locales", mango.WrapErrorHandler(h.ListLocales))
	r.Get("/api/locales/{code}", mango.WrapErrorHandler(h.GetLocale))
	r.Get("/api/themes/{theme_id}", mango.WrapErrorHandler(h.GetTheme))
	r.Get("/share/{token}", mango.WrapErrorHandler(h.ViewSharedNote))

	r.Mount("/static", http.StripPrefix("/static", http.FileServer(http.Dir(params.StaticDir))))

	r.Route("/api", func(api chi.Router) {
		api.Use(h.authMiddleware)
		api.Get("/config", mango.WrapErrorHandler(h.GetConfig))
		api.Get("/themes", mango.WrapErrorHandler(h.ListThemes))
		api.Get("/notes", mango.WrapErrorHandler(h.ListNotes))
		api.Get("/notes/{note_path}", mango.WrapErrorHandler(h.GetNote))
		api.Post("/notes/{note_path}", mango.WrapErrorHandler(h.UpsertNote))
		api.Delete("/notes/{note_path}", mango.WrapErrorHandler(h.DeleteNote))
		api.Post("/notes/move", mango.WrapErrorHandler(h.MoveNote))

		api.Post("/folders", mango.WrapErrorHandler(h.CreateFolder))
		api.Post("/folders/move", mango.WrapErrorHandler(h.MoveFolder))
		api.Post("/folders/rename", mango.WrapErrorHandler(h.RenameFolder))
		api.Delete("/folders/{folder_path}", mango.WrapErrorHandler(h.DeleteFolder))

		api.Get("/media/{media_path}", mango.WrapErrorHandler(h.GetMedia))
		api.Post("/upload-media", mango.WrapErrorHandler(h.UploadMedia))
		api.Post("/media/move", mango.WrapErrorHandler(h.MoveMedia))

		api.Get("/search", mango.WrapErrorHandler(h.Search))
		api.Get("/graph", mango.WrapErrorHandler(h.Graph))

		api.Get("/tags", mango.WrapErrorHandler(h.ListTags))
		api.Get("/tags/{tag_name}", mango.WrapErrorHandler(h.NotesByTag))

		api.Get("/templates", mango.WrapErrorHandler(h.ListTemplates))
		api.Get("/templates/{template_name}", mango.WrapErrorHandler(h.GetTemplate))
		api.Post("/templates/create-note", mango.WrapErrorHandler(h.CreateFromTemplate))

		api.Get("/plugins", mango.WrapErrorHandler(h.ListPlugins))
		api.Get("/plugins/note_stats/calculate", mango.WrapErrorHandler(h.CalculateNoteStats))
		api.Post("/plugins/{plugin_name}/toggle", mango.WrapErrorHandler(h.TogglePlugin))

		api.Post("/share/{note_path}", mango.WrapErrorHandler(h.CreateShare))
		api.Get("/share/{note_path}", mango.WrapErrorHandler(h.GetShareStatus))
		api.Delete("/share/{note_path}", mango.WrapErrorHandler(h.DeleteShare))
		api.Get("/shared-notes", mango.WrapErrorHandler(h.ListSharedNotes))
	})

	r.Group(func(pr chi.Router) {
		pr.Use(h.authMiddleware)
		pr.Get("/*", mango.WrapErrorHandler(h.CatchAll))
		pr.Get("/", mango.WrapErrorHandler(h.CatchAll))
	})

	return &Server{
		srv: &http.Server{
			Addr:         fmt.Sprintf(":%d", params.Config.Server.Port),
			Handler:      r,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		port:     params.Config.Server.Port,
		handlers: h,
	}
}

func (s *Server) Run() error {
	zone, offset := time.Now().Zone()
	slog.Info("starting server", "port", s.port, "timezone", fmt.Sprintf("%s (UTC%+d)", zone, offset/3600))
	return s.srv.ListenAndServe()
}

func (s *Server) Close() error {
	return s.srv.Close()
}

func (h *Handlers) corsMiddleware(next http.Handler) http.Handler {
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

func (h *Handlers) authMiddleware(next http.Handler) http.Handler {
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

func readVersion(versionFile string) string {
	buf, err := os.ReadFile(versionFile)
	if err != nil {
		return "0.0.0"
	}
	return strings.TrimSpace(string(buf))
}

func defaultStaticDir() string {
	return filepath.Clean("frontend")
}
