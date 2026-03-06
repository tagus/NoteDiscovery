package graph

import (
	"path/filepath"
	"regexp"
	"strings"

	"notediscovery/internal/notes"
)

type Service struct {
	notes *notes.Service
}

func NewService(notesSvc *notes.Service) *Service {
	return &Service{notes: notesSvc}
}

func (s *Service) Build() (map[string]any, error) {
	allNotes, _, err := s.notes.ListNotes(false)
	if err != nil {
		return nil, err
	}
	nodes := []map[string]any{}
	edges := []map[string]any{}
	notePaths := map[string]string{}
	noteNames := map[string]string{}

	for _, note := range allNotes {
		nodes = append(nodes, map[string]any{"id": note.Path, "label": note.Name})
		notePaths[strings.ToLower(note.Path)] = note.Path
		notePaths[strings.ToLower(strings.TrimSuffix(note.Path, ".md"))] = note.Path
		noteNames[strings.ToLower(note.Name)] = note.Path
	}

	wikiRE := regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`)
	mdRE := regexp.MustCompile(`\[[^\]]+\]\((?!https?://|mailto:|#|data:)([^\)]+)\)`)
	seen := map[string]struct{}{}

	for _, note := range allNotes {
		content, err := s.notes.ReadNote(note.Path)
		if err != nil {
			continue
		}
		for _, match := range wikiRE.FindAllStringSubmatch(content, -1) {
			target := strings.ToLower(strings.TrimSpace(match[1]))
			targetPath := notePaths[target]
			if targetPath == "" && !strings.HasSuffix(target, ".md") {
				targetPath = notePaths[target+".md"]
			}
			if targetPath == "" {
				targetPath = noteNames[filepath.Base(target)]
			}
			s.addEdge(note.Path, targetPath, "wikilink", seen, &edges)
		}
		for _, match := range mdRE.FindAllStringSubmatch(content, -1) {
			target := strings.TrimSpace(strings.Split(match[1], "#")[0])
			if target == "" {
				continue
			}
			target = strings.TrimPrefix(target, "./")
			lower := strings.ToLower(target)
			targetPath := notePaths[lower]
			if targetPath == "" && !strings.HasSuffix(lower, ".md") {
				targetPath = notePaths[lower+".md"]
			}
			if targetPath == "" {
				targetPath = noteNames[strings.ToLower(strings.TrimSuffix(filepath.Base(target), ".md"))]
			}
			s.addEdge(note.Path, targetPath, "markdown", seen, &edges)
		}
	}

	return map[string]any{"nodes": nodes, "edges": edges}, nil
}

func (s *Service) addEdge(source string, target string, typ string, seen map[string]struct{}, edges *[]map[string]any) {
	if source == "" || target == "" || source == target {
		return
	}
	k := source + "::" + target
	if _, ok := seen[k]; ok {
		return
	}
	seen[k] = struct{}{}
	*edges = append(*edges, map[string]any{"source": source, "target": target, "type": typ})
}
