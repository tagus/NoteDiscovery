package notes

import (
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

var (
	mediaExtensions = map[string]string{
		".jpg": "image", ".jpeg": "image", ".png": "image", ".gif": "image", ".webp": "image",
		".mp3": "audio", ".wav": "audio", ".ogg": "audio", ".m4a": "audio",
		".mp4": "video", ".webm": "video", ".mov": "video", ".avi": "video",
		".pdf": "document",
	}
	tagPattern = regexp.MustCompile(`#([a-zA-Z0-9_-]+)`) // fallback inline tags
)

type Service struct {
	notesDir string
}

type Note struct {
	Name     string   `json:"name"`
	Path     string   `json:"path"`
	Folder   string   `json:"folder"`
	Modified string   `json:"modified"`
	Size     int64    `json:"size"`
	Type     string   `json:"type"`
	Tags     []string `json:"tags"`
}

type NoteMetadata struct {
	Created  string `json:"created"`
	Modified string `json:"modified"`
	Size     int64  `json:"size"`
	Lines    int    `json:"lines"`
}

type SearchMatch struct {
	LineNumber int    `json:"line_number"`
	Context    string `json:"context"`
}

type SearchResult struct {
	Name    string        `json:"name"`
	Path    string        `json:"path"`
	Folder  string        `json:"folder"`
	Matches []SearchMatch `json:"matches"`
}

type Template struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	Modified string `json:"modified"`
}

func NewService(notesDir string) *Service {
	return &Service{notesDir: notesDir}
}

func (s *Service) EnsureDirectories(pluginsDir string) error {
	if err := os.MkdirAll(s.notesDir, 0o755); err != nil {
		return err
	}
	return os.MkdirAll(pluginsDir, 0o755)
}

func (s *Service) IsSecurePath(p string) bool {
	base, err := filepath.Abs(s.notesDir)
	if err != nil {
		return false
	}
	target, err := filepath.Abs(filepath.Join(s.notesDir, p))
	if err != nil {
		return false
	}
	return target == base || strings.HasPrefix(target, base+string(os.PathSeparator))
}

func (s *Service) ListNotes(includeMedia bool) ([]Note, []string, error) {
	notes := []Note{}
	folderSet := map[string]struct{}{}

	err := filepath.WalkDir(s.notesDir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(s.notesDir, path)
		if err != nil {
			return nil
		}
		rel = filepath.ToSlash(rel)

		if strings.HasPrefix(d.Name(), ".") && rel != "." {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		if d.IsDir() {
			if rel != "." {
				folderSet[rel] = struct{}{}
			}
			return nil
		}

		ext := strings.ToLower(filepath.Ext(d.Name()))
		mediaType := mediaExtensions[ext]
		isMarkdown := ext == ".md"
		if !isMarkdown && !(includeMedia && mediaType != "") {
			return nil
		}

		st, err := d.Info()
		if err != nil {
			return nil
		}
		folder := filepath.ToSlash(filepath.Dir(rel))
		if folder == "." {
			folder = ""
		}
		tags := []string{}
		if isMarkdown {
			tags, _ = s.readTags(rel)
		}
		noteType := "note"
		if mediaType != "" {
			noteType = mediaType
		}
		notes = append(notes, Note{
			Name:     strings.TrimSuffix(d.Name(), ext),
			Path:     rel,
			Folder:   folder,
			Modified: st.ModTime().UTC().Format(time.RFC3339),
			Size:     st.Size(),
			Type:     noteType,
			Tags:     tags,
		})
		return nil
	})
	if err != nil {
		return nil, nil, err
	}

	sort.Slice(notes, func(i, j int) bool { return notes[i].Modified > notes[j].Modified })
	folders := make([]string, 0, len(folderSet))
	for folder := range folderSet {
		folders = append(folders, folder)
	}
	sort.Strings(folders)
	return notes, folders, nil
}

func (s *Service) ReadNote(notePath string) (string, error) {
	notePath = s.normalizeNotePath(notePath)
	if !s.IsSecurePath(notePath) {
		return "", os.ErrPermission
	}
	buf, err := os.ReadFile(filepath.Join(s.notesDir, filepath.FromSlash(notePath)))
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func (s *Service) WriteNote(notePath string, content string) (string, error) {
	notePath = s.normalizeNotePath(notePath)
	if !s.IsSecurePath(notePath) {
		return "", os.ErrPermission
	}
	fullPath := filepath.Join(s.notesDir, filepath.FromSlash(notePath))
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(fullPath, []byte(content), 0o644); err != nil {
		return "", err
	}
	return notePath, nil
}

func (s *Service) DeleteNote(notePath string) error {
	notePath = s.normalizeNotePath(notePath)
	if !s.IsSecurePath(notePath) {
		return os.ErrPermission
	}
	return os.Remove(filepath.Join(s.notesDir, filepath.FromSlash(notePath)))
}

func (s *Service) MoveFile(oldPath string, newPath string, requireMedia bool) error {
	if !s.IsSecurePath(oldPath) || !s.IsSecurePath(newPath) {
		return os.ErrPermission
	}
	if requireMedia {
		ext := strings.ToLower(filepath.Ext(oldPath))
		if _, ok := mediaExtensions[ext]; !ok {
			return fmt.Errorf("not media file")
		}
	}
	oldFull := filepath.Join(s.notesDir, filepath.FromSlash(oldPath))
	newFull := filepath.Join(s.notesDir, filepath.FromSlash(newPath))
	if _, err := os.Stat(newFull); err == nil {
		return fmt.Errorf("target exists")
	}
	if err := os.MkdirAll(filepath.Dir(newFull), 0o755); err != nil {
		return err
	}
	return os.Rename(oldFull, newFull)
}

func (s *Service) CreateFolder(folder string) error {
	if !s.IsSecurePath(folder) {
		return os.ErrPermission
	}
	return os.MkdirAll(filepath.Join(s.notesDir, filepath.FromSlash(folder)), 0o755)
}

func (s *Service) MoveFolder(oldPath string, newPath string) error {
	if !s.IsSecurePath(oldPath) || !s.IsSecurePath(newPath) {
		return os.ErrPermission
	}
	oldFull := filepath.Join(s.notesDir, filepath.FromSlash(oldPath))
	newFull := filepath.Join(s.notesDir, filepath.FromSlash(newPath))
	if err := os.MkdirAll(filepath.Dir(newFull), 0o755); err != nil {
		return err
	}
	return os.Rename(oldFull, newFull)
}

func (s *Service) DeleteFolder(folder string) error {
	if !s.IsSecurePath(folder) {
		return os.ErrPermission
	}
	return os.RemoveAll(filepath.Join(s.notesDir, filepath.FromSlash(folder)))
}

func (s *Service) StatNote(notePath string) (*NoteMetadata, error) {
	notePath = s.normalizeNotePath(notePath)
	full := filepath.Join(s.notesDir, filepath.FromSlash(notePath))
	st, err := os.Stat(full)
	if err != nil {
		return nil, err
	}
	buf, err := os.ReadFile(full)
	if err != nil {
		return nil, err
	}
	return &NoteMetadata{
		Created:  st.ModTime().UTC().Format(time.RFC3339),
		Modified: st.ModTime().UTC().Format(time.RFC3339),
		Size:     st.Size(),
		Lines:    strings.Count(string(buf), "\n") + 1,
	}, nil
}

func (s *Service) Search(query string) ([]SearchResult, error) {
	if strings.TrimSpace(query) == "" {
		return []SearchResult{}, nil
	}
	allNotes, _, err := s.ListNotes(false)
	if err != nil {
		return nil, err
	}
	results := []SearchResult{}
	re := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(query))

	for _, note := range allNotes {
		content, err := s.ReadNote(note.Path)
		if err != nil {
			continue
		}
		locs := re.FindAllStringIndex(content, 3)
		if len(locs) == 0 {
			continue
		}
		matches := make([]SearchMatch, 0, len(locs))
		for _, loc := range locs {
			start, end := loc[0], loc[1]
			left := max(0, start-15)
			right := min(len(content), end+15)
			before := html.EscapeString(strings.ReplaceAll(content[left:start], "\n", " "))
			after := html.EscapeString(strings.ReplaceAll(content[end:right], "\n", " "))
			hit := html.EscapeString(strings.ReplaceAll(content[start:end], "\n", " "))
			snippet := before + `<mark class="search-highlight">` + hit + `</mark>` + after
			if left > 0 {
				snippet = "..." + snippet
			}
			if right < len(content) {
				snippet += "..."
			}
			matches = append(matches, SearchMatch{LineNumber: strings.Count(content[:start], "\n") + 1, Context: snippet})
		}
		results = append(results, SearchResult{Name: note.Name, Path: note.Path, Folder: note.Folder, Matches: matches})
	}
	return results, nil
}

func (s *Service) ParseTags(content string) []string {
	if !strings.HasPrefix(strings.TrimSpace(content), "---") {
		return []string{}
	}
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return []string{}
	}
	frontmatter := []string{}
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			break
		}
		frontmatter = append(frontmatter, lines[i])
	}
	tags := map[string]struct{}{}
	inTagList := false
	for _, line := range frontmatter {
		trim := strings.TrimSpace(line)
		if strings.HasPrefix(trim, "tags:") {
			rest := strings.TrimSpace(strings.TrimPrefix(trim, "tags:"))
			if strings.HasPrefix(rest, "[") && strings.HasSuffix(rest, "]") {
				parts := strings.Split(strings.TrimSuffix(strings.TrimPrefix(rest, "["), "]"), ",")
				for _, part := range parts {
					t := strings.ToLower(strings.TrimSpace(part))
					if t != "" {
						tags[t] = struct{}{}
					}
				}
				continue
			}
			if rest == "" {
				inTagList = true
				continue
			}
			tags[strings.ToLower(rest)] = struct{}{}
			continue
		}
		if inTagList {
			if strings.HasPrefix(trim, "-") {
				t := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(trim, "-")))
				if t != "" {
					tags[t] = struct{}{}
				}
			} else if trim != "" {
				inTagList = false
			}
		}
	}
	for _, match := range tagPattern.FindAllStringSubmatch(content, -1) {
		tags[strings.ToLower(match[1])] = struct{}{}
	}
	out := make([]string, 0, len(tags))
	for tag := range tags {
		out = append(out, tag)
	}
	sort.Strings(out)
	return out
}

func (s *Service) AllTags() (map[string]int, error) {
	notes, _, err := s.ListNotes(false)
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, note := range notes {
		seen := map[string]struct{}{}
		for _, tag := range note.Tags {
			if _, ok := seen[tag]; ok {
				continue
			}
			out[tag]++
			seen[tag] = struct{}{}
		}
	}
	return out, nil
}

func (s *Service) NotesByTag(tag string) ([]Note, error) {
	notes, _, err := s.ListNotes(false)
	if err != nil {
		return nil, err
	}
	tag = strings.ToLower(strings.TrimSpace(tag))
	out := []Note{}
	for _, note := range notes {
		for _, t := range note.Tags {
			if t == tag {
				out = append(out, note)
				break
			}
		}
	}
	return out, nil
}

func (s *Service) ListTemplates() ([]Template, error) {
	templatesDir := filepath.Join(s.notesDir, "_templates")
	entries, err := os.ReadDir(templatesDir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return []Template{}, nil
		}
		return nil, err
	}
	result := []Template{}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(strings.ToLower(entry.Name()), ".md") {
			continue
		}
		st, err := entry.Info()
		if err != nil {
			continue
		}
		result = append(result, Template{
			Name:     strings.TrimSuffix(entry.Name(), ".md"),
			Path:     "_templates/" + entry.Name(),
			Modified: st.ModTime().UTC().Format(time.RFC3339),
		})
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result, nil
}

func (s *Service) GetTemplateContent(name string) (string, error) {
	full := filepath.Join(s.notesDir, "_templates", name+".md")
	buf, err := os.ReadFile(full)
	if err != nil {
		return "", err
	}
	return string(buf), nil
}

func (s *Service) ApplyTemplatePlaceholders(content string, notePath string) string {
	now := time.Now()
	title := strings.TrimSuffix(filepath.Base(notePath), filepath.Ext(notePath))
	folder := strings.Trim(filepath.ToSlash(filepath.Dir(notePath)), ".")
	replacements := map[string]string{
		"{{date}}":      now.Format("2006-01-02"),
		"{{time}}":      now.Format("15:04:05"),
		"{{datetime}}":  now.Format(time.RFC3339),
		"{{timestamp}}": fmt.Sprintf("%d", now.Unix()),
		"{{year}}":      now.Format("2006"),
		"{{month}}":     now.Format("01"),
		"{{day}}":       now.Format("02"),
		"{{title}}":     title,
		"{{folder}}":    folder,
	}
	for key, value := range replacements {
		content = strings.ReplaceAll(content, key, value)
	}
	return content
}

func (s *Service) SaveUploadedMedia(notePath string, fileHeader *multipart.FileHeader, file multipart.File) (string, string, error) {
	ext := strings.ToLower(filepath.Ext(fileHeader.Filename))
	mediaType, ok := mediaExtensions[ext]
	if !ok {
		return "", "", fmt.Errorf("unsupported media type")
	}
	limits := map[string]int64{"image": 10 << 20, "audio": 50 << 20, "video": 100 << 20, "document": 20 << 20}
	maxSize := limits[mediaType]

	buf, err := io.ReadAll(io.LimitReader(file, maxSize+1))
	if err != nil {
		return "", "", err
	}
	if int64(len(buf)) > maxSize {
		return "", "", fmt.Errorf("file too large")
	}

	safeName := sanitizeFilename(fileHeader.Filename)
	stamp := time.Now().Format("20060102150405")
	base := strings.TrimSuffix(safeName, filepath.Ext(safeName))
	filename := fmt.Sprintf("%s-%s%s", base, stamp, ext)

	folder := filepath.ToSlash(filepath.Dir(notePath))
	if folder == "." {
		folder = ""
	}
	attachments := "_attachments"
	if folder != "" {
		attachments = folder + "/_attachments"
	}
	if !s.IsSecurePath(attachments + "/" + filename) {
		return "", "", os.ErrPermission
	}
	fullDir := filepath.Join(s.notesDir, filepath.FromSlash(attachments))
	if err := os.MkdirAll(fullDir, 0o755); err != nil {
		return "", "", err
	}
	rel := attachments + "/" + filename
	if err := os.WriteFile(filepath.Join(s.notesDir, filepath.FromSlash(rel)), buf, 0o644); err != nil {
		return "", "", err
	}
	return rel, mediaType, nil
}

func (s *Service) ServeMedia(w http.ResponseWriter, r *http.Request, mediaPath string) error {
	if !s.IsSecurePath(mediaPath) {
		return os.ErrPermission
	}
	ext := strings.ToLower(filepath.Ext(mediaPath))
	if _, ok := mediaExtensions[ext]; !ok {
		return fmt.Errorf("not media")
	}
	http.ServeFile(w, r, filepath.Join(s.notesDir, filepath.FromSlash(mediaPath)))
	return nil
}

func (s *Service) LoadShareTokens() (map[string]map[string]any, error) {
	path := filepath.Join(s.notesDir, ".share-tokens.json")
	buf, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return map[string]map[string]any{}, nil
		}
		return nil, err
	}
	tokens := map[string]map[string]any{}
	if err := json.Unmarshal(buf, &tokens); err != nil {
		return map[string]map[string]any{}, nil
	}
	return tokens, nil
}

func (s *Service) SaveShareTokens(tokens map[string]map[string]any) error {
	buf, err := json.MarshalIndent(tokens, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(s.notesDir, ".share-tokens.json"), buf, 0o644)
}

func (s *Service) readTags(notePath string) ([]string, error) {
	content, err := s.ReadNote(notePath)
	if err != nil {
		return nil, err
	}
	return s.ParseTags(content), nil
}

func (s *Service) normalizeNotePath(notePath string) string {
	unescaped := notePath
	if decoded, err := urlPathUnescape(notePath); err == nil {
		unescaped = decoded
	}
	if !strings.HasSuffix(strings.ToLower(unescaped), ".md") {
		unescaped += ".md"
	}
	return strings.TrimPrefix(filepath.ToSlash(filepath.Clean(unescaped)), "./")
}

func sanitizeFilename(filename string) string {
	filename = strings.TrimSpace(filename)
	if filename == "" {
		return "unnamed"
	}
	re := regexp.MustCompile(`[\\/:*?"<>|\x00-\x1f]`)
	filename = re.ReplaceAllString(filename, "_")
	filename = strings.Trim(filename, "_ ")
	if filename == "" {
		return "unnamed"
	}
	return filename
}

func urlPathUnescape(s string) (string, error) {
	r := strings.NewReplacer("+", "%2B")
	return url.QueryUnescape(r.Replace(s))
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
