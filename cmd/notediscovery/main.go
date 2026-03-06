package main

import (
	"flag"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"notediscovery/internal/auth"
	"notediscovery/internal/config"
	"notediscovery/internal/graph"
	"notediscovery/internal/locales"
	"notediscovery/internal/notes"
	"notediscovery/internal/plugins"
	"notediscovery/internal/server"
	"notediscovery/internal/share"
	"notediscovery/internal/themes"

	"github.com/tagus/mango"
)

var (
	configPath = flag.String("config", "config.yaml", "path to config file")
	port       = flag.Int("port", 0, "port override")
)

func main() {
	flag.Parse()
	mango.Init(slog.LevelDebug, mango.LoggerModeDebug, "notediscovery")

	cfg, err := config.Load(*configPath)
	mango.FatalIf(err)

	if *port != 0 {
		cfg.Server.Port = *port
	}
	if envPort := os.Getenv("PORT"); envPort != "" {
		if p, err := mango.ParseInt(envPort); err == nil {
			cfg.Server.Port = p
		}
	}
	if v, err := os.ReadFile("VERSION"); err == nil {
		cfg.App.Version = strings.TrimSpace(string(v))
	}

	if env := os.Getenv("AUTHENTICATION_ENABLED"); env != "" {
		cfg.Authentication.Enabled = strings.EqualFold(env, "true") || env == "1" || strings.EqualFold(env, "yes")
	}
	if env := os.Getenv("AUTHENTICATION_SECRET_KEY"); env != "" {
		cfg.Authentication.SecretKey = env
	}
	if env := os.Getenv("AUTHENTICATION_PASSWORD_HASH"); env != "" {
		cfg.Authentication.PasswordHash = env
	}
	if env := os.Getenv("AUTHENTICATION_PASSWORD"); env != "" {
		hash, err := auth.HashPassword(env)
		mango.FatalIf(err)
		cfg.Authentication.PasswordHash = hash
	}
	if cfg.Authentication.PasswordHash == "" && cfg.Authentication.Password != "" {
		hash, err := auth.HashPassword(cfg.Authentication.Password)
		mango.FatalIf(err)
		cfg.Authentication.PasswordHash = hash
	}

	notesSvc := notes.NewService(cfg.Storage.NotesDir)
	mango.FatalIf(notesSvc.EnsureDirectories(cfg.Storage.PluginsDir))
	themesSvc := themes.NewService("themes")
	localesSvc := locales.NewService("locales")
	pluginsSvc := plugins.NewService(cfg.Storage.PluginsDir)
	graphSvc := graph.NewService(notesSvc)
	shareSvc := share.NewService(notesSvc, "themes")
	authSvc := auth.NewService(cfg.Authentication.SecretKey, cfg.Authentication.PasswordHash, cfg.Authentication.Enabled, cfg.Authentication.SessionMaxAge)

	srv := server.New(server.Params{
		Config:     cfg,
		Version:    cfg.App.Version,
		StaticDir:  "frontend",
		ThemesDir:  "themes",
		LocalesDir: "locales",
		Notes:      notesSvc,
		Themes:     themesSvc,
		Locales:    localesSvc,
		Plugins:    pluginsSvc,
		Graph:      graphSvc,
		Share:      shareSvc,
		Auth:       authSvc,
	})

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		mango.ErrorIf(srv.Close())
	}()

	mango.FatalIf(srv.Run())
}
