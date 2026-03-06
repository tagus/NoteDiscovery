package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type App struct {
	Name    string `yaml:"name"`
	Version string `yaml:"version"`
}

type Server struct {
	Host           string   `yaml:"host"`
	Port           int      `yaml:"port"`
	Reload         bool     `yaml:"reload"`
	AllowedOrigins []string `yaml:"allowed_origins"`
	Debug          bool     `yaml:"debug"`
}

type Storage struct {
	NotesDir   string `yaml:"notes_dir"`
	PluginsDir string `yaml:"plugins_dir"`
}

type Search struct {
	Enabled bool `yaml:"enabled"`
}

type Authentication struct {
	Enabled       bool   `yaml:"enabled"`
	SecretKey     string `yaml:"secret_key"`
	Password      string `yaml:"password"`
	PasswordHash  string `yaml:"password_hash"`
	SessionMaxAge int    `yaml:"session_max_age"`
}

type Config struct {
	App            App            `yaml:"app"`
	Server         Server         `yaml:"server"`
	Storage        Storage        `yaml:"storage"`
	Search         Search         `yaml:"search"`
	Authentication Authentication `yaml:"authentication"`
}

func Load(cfgPath string) (*Config, error) {
	buf, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(buf, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	if cfg.Server.Host == "" {
		cfg.Server.Host = "0.0.0.0"
	}
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8000
	}
	if len(cfg.Server.AllowedOrigins) == 0 {
		cfg.Server.AllowedOrigins = []string{"*"}
	}
	if cfg.Storage.NotesDir == "" {
		cfg.Storage.NotesDir = "./data"
	}
	if cfg.Storage.PluginsDir == "" {
		cfg.Storage.PluginsDir = "./plugins"
	}
	if cfg.Authentication.SessionMaxAge == 0 {
		cfg.Authentication.SessionMaxAge = 604800
	}

	return &cfg, nil
}
