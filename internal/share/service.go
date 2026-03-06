package share

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"strings"
	"time"

	"notediscovery/internal/notes"
)

type Service struct {
	notes     *notes.Service
	themesDir string
}

type ShareInfo struct {
	Path    string `json:"path"`
	Theme   string `json:"theme"`
	Created string `json:"created"`
}

func NewService(notesSvc *notes.Service, themesDir string) *Service {
	return &Service{notes: notesSvc, themesDir: themesDir}
}

func (s *Service) CreateToken(notePath string, theme string) (string, error) {
	tokens, err := s.notes.LoadShareTokens()
	if err != nil {
		return "", err
	}
	for token, info := range tokens {
		if infoPath, _ := info["path"].(string); infoPath == notePath {
			return token, nil
		}
	}
	buf := make([]byte, 12)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	token := base64.RawURLEncoding.EncodeToString(buf)
	tokens[token] = map[string]any{"path": notePath, "theme": theme, "created": time.Now().UTC().Format(time.RFC3339)}
	return token, s.notes.SaveShareTokens(tokens)
}

func (s *Service) InfoForPath(notePath string) (map[string]any, error) {
	tokens, err := s.notes.LoadShareTokens()
	if err != nil {
		return nil, err
	}
	for token, info := range tokens {
		if infoPath, _ := info["path"].(string); infoPath == notePath {
			return map[string]any{"shared": true, "token": token, "theme": info["theme"], "created": info["created"]}, nil
		}
	}
	return map[string]any{"shared": false}, nil
}

func (s *Service) RevokeByPath(notePath string) (bool, error) {
	tokens, err := s.notes.LoadShareTokens()
	if err != nil {
		return false, err
	}
	for token, info := range tokens {
		if infoPath, _ := info["path"].(string); infoPath == notePath {
			delete(tokens, token)
			return true, s.notes.SaveShareTokens(tokens)
		}
	}
	return false, nil
}

func (s *Service) UpdatePath(oldPath string, newPath string) error {
	tokens, err := s.notes.LoadShareTokens()
	if err != nil {
		return err
	}
	changed := false
	for _, info := range tokens {
		if infoPath, _ := info["path"].(string); infoPath == oldPath {
			info["path"] = newPath
			changed = true
		}
	}
	if !changed {
		return nil
	}
	return s.notes.SaveShareTokens(tokens)
}

func (s *Service) Paths() ([]string, error) {
	tokens, err := s.notes.LoadShareTokens()
	if err != nil {
		return nil, err
	}
	paths := []string{}
	for _, info := range tokens {
		if p, ok := info["path"].(string); ok && p != "" {
			paths = append(paths, p)
		}
	}
	return paths, nil
}

func (s *Service) NoteByToken(token string) (*ShareInfo, error) {
	tokens, err := s.notes.LoadShareTokens()
	if err != nil {
		return nil, err
	}
	info, ok := tokens[token]
	if !ok {
		return nil, fmt.Errorf("not found")
	}
	path, _ := info["path"].(string)
	theme, _ := info["theme"].(string)
	created, _ := info["created"].(string)
	return &ShareInfo{Path: path, Theme: theme, Created: created}, nil
}

func StripFrontmatter(content string) string {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return content
	}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			return strings.Join(lines[i+1:], "\n")
		}
	}
	return content
}

func (s *Service) RenderSharedHTML(title string, body string, theme string) string {
	themeCSS := ""
	if theme != "" {
		if buf, err := os.ReadFile(filepath.Join(s.themesDir, theme+".css")); err == nil {
			themeCSS = string(buf)
		}
	}
	if themeCSS == "" {
		if buf, err := os.ReadFile(filepath.Join(s.themesDir, "light.css")); err == nil {
			themeCSS = string(buf)
		}
	}
	body = template.HTMLEscapeString(body)
	tpl := `<!DOCTYPE html><html><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1"><title>{{.Title}}</title><style>{{.Theme}}</style><style>body{max-width:900px;margin:2rem auto;padding:0 1rem;font-family:system-ui}pre{white-space:pre-wrap}</style></head><body><h1>{{.Title}}</h1><pre>{{.Body}}</pre></body></html>`
	return strings.NewReplacer("{{.Title}}", template.HTMLEscapeString(title), "{{.Theme}}", themeCSS, "{{.Body}}", body).Replace(tpl)
}
