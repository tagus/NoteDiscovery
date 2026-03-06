package themes

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type Service struct {
	dir string
}

type Theme struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Builtin bool   `json:"builtin"`
}

func NewService(dir string) *Service {
	return &Service{dir: dir}
}

func (s *Service) List() ([]Theme, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}
	icons := map[string]string{
		"light": "🌞", "dark": "🌙", "dracula": "🧛", "nord": "❄️", "monokai": "🎞️", "vue-high-contrast": "💚", "cobalt2": "🌊", "vs-blue": "🔷", "gruvbox-dark": "🟫", "matcha-light": "🍵",
	}
	items := []Theme{}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".css" {
			continue
		}
		id := strings.TrimSuffix(entry.Name(), ".css")
		typeName := "dark"
		css, _ := s.GetCSS(id)
		re := regexp.MustCompile(`@theme-type:\s*(light|dark)`)
		if m := re.FindStringSubmatch(css); len(m) > 1 {
			typeName = m[1]
		}
		name := strings.Title(strings.ReplaceAll(strings.ReplaceAll(id, "-", " "), "_", " "))
		items = append(items, Theme{ID: id, Name: icons[id] + " " + name, Type: typeName, Builtin: false})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func (s *Service) GetCSS(themeID string) (string, error) {
	buf, err := os.ReadFile(filepath.Join(s.dir, themeID+".css"))
	if err != nil {
		return "", err
	}
	return string(buf), nil
}
