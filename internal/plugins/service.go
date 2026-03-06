package plugins

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"

	"github.com/go-chi/chi/v5"
	"github.com/tagus/mango"
)

const (
	HookOnNoteCreate = "on_note_create"
	HookOnNoteSave   = "on_note_save"
	HookOnNoteLoad   = "on_note_load"
	HookOnNoteDelete = "on_note_delete"
	HookOnSearch     = "on_search"
	HookOnAppStartup = "on_app_startup"
)

type Descriptor struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Enabled      bool     `json:"enabled"`
	Capabilities []string `json:"capabilities,omitempty"`
}

type Plugin interface {
	ID() string
	Name() string
	DefaultEnabled() bool
	Capabilities() []string
}

type ContentAnalyzer interface {
	Analyze(content string) (any, error)
}

type NoteCreateHook interface {
	OnNoteCreate(notePath string, initialContent string) (string, error)
}

type NoteSaveHook interface {
	OnNoteSave(notePath string, content string) (*string, error)
}

type NoteLoadHook interface {
	OnNoteLoad(notePath string, content string) (*string, error)
}

type NoteDeleteHook interface {
	OnNoteDelete(notePath string) error
}

type SearchHook interface {
	OnSearch(query string, results any) error
}

type AppStartupHook interface {
	OnAppStartup() error
}

type RouteInstaller interface {
	InstallRoutes(r chi.Router, svc *Service)
}

type runtimePlugin struct {
	impl    Plugin
	enabled bool
}

type Service struct {
	dir     string
	plugins map[string]*runtimePlugin
}

func NewService(dir string) *Service {
	s := &Service{
		dir:     dir,
		plugins: map[string]*runtimePlugin{},
	}

	// Built-ins
	s.Register(NewNoteStatsPlugin())
	s.loadState()

	return s
}

func (s *Service) Register(plugin Plugin) {
	s.plugins[plugin.ID()] = &runtimePlugin{impl: plugin, enabled: plugin.DefaultEnabled()}
}

func (s *Service) List() []Descriptor {
	items := make([]Descriptor, 0, len(s.plugins))
	for id, runtime := range s.plugins {
		items = append(items, Descriptor{
			ID:           id,
			Name:         runtime.impl.Name(),
			Enabled:      runtime.enabled,
			Capabilities: runtime.impl.Capabilities(),
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items
}

func (s *Service) Toggle(name string, enabled bool) bool {
	runtime, ok := s.plugins[name]
	if !ok {
		return false
	}
	runtime.enabled = enabled
	s.saveState()
	return true
}

func (s *Service) Enabled(name string) bool {
	runtime, ok := s.plugins[name]
	if !ok {
		return false
	}
	return runtime.enabled
}

func (s *Service) AnalyzeContent(name string, content string) (any, bool, bool) {
	runtime, ok := s.plugins[name]
	if !ok {
		return nil, false, false
	}
	if !runtime.enabled {
		return nil, false, true
	}
	analyzer, ok := runtime.impl.(ContentAnalyzer)
	if !ok {
		return nil, false, true
	}
	payload, err := analyzer.Analyze(content)
	if err != nil {
		return nil, false, true
	}
	return payload, true, true
}

func (s *Service) InstallRoutes(r chi.Router) {
	h := NewHandler(s)
	r.Get("/plugins", mango.WrapErrorHandler(h.List))
	r.Post("/plugins/{plugin_name}/toggle", mango.WrapErrorHandler(h.Toggle))

	for _, runtime := range s.plugins {
		installer, ok := runtime.impl.(RouteInstaller)
		if !ok {
			continue
		}
		installer.InstallRoutes(r, s)
	}
}

// RunHook executes lifecycle hooks using a Python-like payload map contract.
// It returns the (potentially mutated) payload.
func (s *Service) RunHook(hookName string, payload map[string]any) map[string]any {
	if payload == nil {
		payload = map[string]any{}
	}

	switch hookName {
	case HookOnAppStartup:
		s.runAppStartup()
	case HookOnNoteCreate:
		notePath, _ := payload["note_path"].(string)
		initialContent, _ := payload["initial_content"].(string)
		payload["initial_content"] = s.runNoteCreate(notePath, initialContent)
	case HookOnNoteSave:
		notePath, _ := payload["note_path"].(string)
		content, _ := payload["content"].(string)
		payload["content"] = s.runNoteSave(notePath, content)
	case HookOnNoteLoad:
		notePath, _ := payload["note_path"].(string)
		content, _ := payload["content"].(string)
		payload["content"] = s.runNoteLoad(notePath, content)
	case HookOnNoteDelete:
		notePath, _ := payload["note_path"].(string)
		s.runNoteDelete(notePath)
	case HookOnSearch:
		query, _ := payload["query"].(string)
		results := payload["results"]
		s.runSearch(query, results)
	}

	return payload
}

func (s *Service) runAppStartup() {
	for _, runtime := range s.plugins {
		if !runtime.enabled {
			continue
		}
		hook, ok := runtime.impl.(AppStartupHook)
		if !ok {
			continue
		}
		_ = hook.OnAppStartup()
	}
}

func (s *Service) runNoteCreate(notePath string, initialContent string) string {
	content := initialContent
	for _, runtime := range s.plugins {
		if !runtime.enabled {
			continue
		}
		hook, ok := runtime.impl.(NoteCreateHook)
		if !ok {
			continue
		}
		updated, err := hook.OnNoteCreate(notePath, content)
		if err != nil {
			continue
		}
		content = updated
	}
	return content
}

func (s *Service) runNoteSave(notePath string, content string) string {
	out := content
	for _, runtime := range s.plugins {
		if !runtime.enabled {
			continue
		}
		hook, ok := runtime.impl.(NoteSaveHook)
		if !ok {
			continue
		}
		updated, err := hook.OnNoteSave(notePath, out)
		if err != nil || updated == nil {
			continue
		}
		out = *updated
	}
	return out
}

func (s *Service) runNoteLoad(notePath string, content string) string {
	out := content
	for _, runtime := range s.plugins {
		if !runtime.enabled {
			continue
		}
		hook, ok := runtime.impl.(NoteLoadHook)
		if !ok {
			continue
		}
		updated, err := hook.OnNoteLoad(notePath, out)
		if err != nil || updated == nil {
			continue
		}
		out = *updated
	}
	return out
}

func (s *Service) runNoteDelete(notePath string) {
	for _, runtime := range s.plugins {
		if !runtime.enabled {
			continue
		}
		hook, ok := runtime.impl.(NoteDeleteHook)
		if !ok {
			continue
		}
		_ = hook.OnNoteDelete(notePath)
	}
}

func (s *Service) runSearch(query string, results any) {
	for _, runtime := range s.plugins {
		if !runtime.enabled {
			continue
		}
		hook, ok := runtime.impl.(SearchHook)
		if !ok {
			continue
		}
		_ = hook.OnSearch(query, results)
	}
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
	if json.Unmarshal(buf, &st) != nil {
		return
	}
	for id, enabled := range st {
		runtime, ok := s.plugins[id]
		if !ok {
			continue
		}
		runtime.enabled = enabled
	}
}

func (s *Service) saveState() {
	state := map[string]bool{}
	for id, runtime := range s.plugins {
		state[id] = runtime.enabled
	}
	buf, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(s.statePath(), buf, 0o644)
}
