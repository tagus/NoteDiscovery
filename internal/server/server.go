package server

import (
	"fmt"
	"log/slog"
	"net/http"
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
	srv  *http.Server
	port int
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
	core := NewCoreHandler(params)
	noteHandler := notes.NewHandler(notes.HandlerParams{
		Config:  params.Config,
		Service: params.Notes,
		Plugins: params.Plugins,
		Share:   params.Share,
	})
	themeHandler := themes.NewHandler(params.Themes)
	localeHandler := locales.NewHandler(params.Locales)
	pluginHandler := params.Plugins
	graphHandler := graph.NewHandler(params.Graph)
	shareHandler := share.NewHandler(params.Share, params.Notes)

	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))
	r.Use(middleware.StripSlashes)
	r.Use(core.corsMiddleware)

	r.Get("/health", mango.WrapErrorHandler(core.Health))
	r.Get("/sw.js", mango.WrapErrorHandler(core.ServiceWorker))
	r.Get("/login", mango.WrapErrorHandler(core.LoginPage))
	r.Post("/login", mango.WrapErrorHandler(core.Login))
	r.Get("/logout", mango.WrapErrorHandler(core.Logout))

	// Public routes needed for login page/app bootstrap.
	r.Get("/api/locales", mango.WrapErrorHandler(localeHandler.List))
	r.Get("/api/locales/{code}", mango.WrapErrorHandler(localeHandler.Get))
	r.Get("/api/themes/{theme_id}", mango.WrapErrorHandler(themeHandler.Get))
	r.Get("/share/{token}", mango.WrapErrorHandler(shareHandler.View))

	r.Mount("/static", http.StripPrefix("/static", http.FileServer(http.Dir(params.StaticDir))))

	r.Route("/api", func(api chi.Router) {
		api.Use(core.authMiddleware)
		api.Get("/config", mango.WrapErrorHandler(core.GetConfig))
		api.Get("/themes", mango.WrapErrorHandler(themeHandler.List))

		api.Get("/notes", mango.WrapErrorHandler(noteHandler.List))
		api.Post("/notes/move", mango.WrapErrorHandler(noteHandler.Move))
		api.Get("/notes/*", mango.WrapErrorHandler(noteHandler.Get))
		api.Post("/notes/*", mango.WrapErrorHandler(noteHandler.Upsert))
		api.Delete("/notes/*", mango.WrapErrorHandler(noteHandler.Delete))

		api.Post("/folders", mango.WrapErrorHandler(noteHandler.CreateFolder))
		api.Post("/folders/move", mango.WrapErrorHandler(noteHandler.MoveFolder))
		api.Post("/folders/rename", mango.WrapErrorHandler(noteHandler.RenameFolder))
		api.Delete("/folders/*", mango.WrapErrorHandler(noteHandler.DeleteFolder))

		api.Get("/media/*", mango.WrapErrorHandler(noteHandler.GetMedia))
		api.Post("/upload-media", mango.WrapErrorHandler(noteHandler.UploadMedia))
		api.Post("/media/move", mango.WrapErrorHandler(noteHandler.MoveMedia))

		api.Get("/search", mango.WrapErrorHandler(noteHandler.Search))
		api.Get("/graph", mango.WrapErrorHandler(graphHandler.Get))

		api.Get("/tags", mango.WrapErrorHandler(noteHandler.ListTags))
		api.Get("/tags/{tag_name}", mango.WrapErrorHandler(noteHandler.NotesByTag))

		api.Get("/templates", mango.WrapErrorHandler(noteHandler.ListTemplates))
		api.Get("/templates/{template_name}", mango.WrapErrorHandler(noteHandler.GetTemplate))
		api.Post("/templates/create-note", mango.WrapErrorHandler(noteHandler.CreateFromTemplate))

		pluginHandler.InstallRoutes(api)

		api.Get("/shared-notes", mango.WrapErrorHandler(shareHandler.List))
		api.Post("/share/*", mango.WrapErrorHandler(shareHandler.Create))
		api.Get("/share/*", mango.WrapErrorHandler(shareHandler.Status))
		api.Delete("/share/*", mango.WrapErrorHandler(shareHandler.Delete))
	})

	r.Group(func(pr chi.Router) {
		pr.Use(core.authMiddleware)
		pr.Get("/*", mango.WrapErrorHandler(core.CatchAll))
		pr.Get("/", mango.WrapErrorHandler(core.CatchAll))
	})

	return &Server{
		srv: &http.Server{
			Addr:         fmt.Sprintf(":%d", params.Config.Server.Port),
			Handler:      r,
			ReadTimeout:  15 * time.Second,
			WriteTimeout: 30 * time.Second,
		},
		port: params.Config.Server.Port,
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
