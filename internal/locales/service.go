package locales

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Service struct {
	dir string
}

type Locale struct {
	Code string `json:"code"`
	Name string `json:"name"`
	Flag string `json:"flag"`
}

func NewService(dir string) *Service {
	return &Service{dir: dir}
}

func (s *Service) List() ([]Locale, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	out := []Locale{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}
		buf, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}
		var raw map[string]any
		if err := json.Unmarshal(buf, &raw); err != nil {
			continue
		}
		meta, _ := raw["_meta"].(map[string]any)
		code, _ := meta["code"].(string)
		name, _ := meta["name"].(string)
		flag, _ := meta["flag"].(string)
		if code == "" {
			code = strings.TrimSuffix(entry.Name(), ".json")
		}
		if name == "" {
			name = code
		}
		if flag == "" {
			flag = "🌐"
		}
		out = append(out, Locale{Code: code, Name: name, Flag: flag})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Code < out[j].Code })
	return out, nil
}

func (s *Service) Get(code string) (map[string]any, error) {
	buf, err := os.ReadFile(filepath.Join(s.dir, code+".json"))
	if err != nil {
		return nil, err
	}
	var raw map[string]any
	if err := json.Unmarshal(buf, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}
