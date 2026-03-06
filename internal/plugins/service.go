package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Service struct {
	dir     string
	plugins map[string]bool
}

type Plugin struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
}

type NoteStats struct {
	Words      int `json:"words"`
	Characters int `json:"characters"`
	Lines      int `json:"lines"`
}

func NewService(dir string) *Service {
	s := &Service{dir: dir, plugins: map[string]bool{"note_stats": true}}
	s.loadState()
	return s
}

func (s *Service) List() []Plugin {
	return []Plugin{{ID: "note_stats", Name: "Note Statistics", Enabled: s.plugins["note_stats"]}}
}

func (s *Service) Toggle(name string, enabled bool) {
	s.plugins[name] = enabled
	s.saveState()
}

func (s *Service) Enabled(name string) bool {
	return s.plugins[name]
}

func (s *Service) Calculate(content string) *NoteStats {
	if !s.Enabled("note_stats") {
		return nil
	}
	trimmed := strings.TrimSpace(content)
	words := 0
	if trimmed != "" {
		words = len(strings.Fields(trimmed))
	}
	return &NoteStats{Words: words, Characters: len(content), Lines: strings.Count(content, "\n") + 1}
}

func (s *Service) statePath() string {
	return filepath.Join(s.dir, ".plugins.json")
}

func (s *Service) loadState() {
	buf, err := os.ReadFile(s.statePath())
	if err != nil {
		return
	}
	var st map[string]bool
	if json.Unmarshal(buf, &st) == nil {
		for k, v := range st {
			s.plugins[k] = v
		}
	}
}

func (s *Service) saveState() {
	buf, err := json.MarshalIndent(s.plugins, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.statePath(), buf, 0o644)
}
