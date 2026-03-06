package plugins

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"

	"github.com/go-chi/chi/v5"
	"github.com/tagus/mango"
)

type headingsStats struct {
	H1    int `json:"h1"`
	H2    int `json:"h2"`
	H3    int `json:"h3"`
	Total int `json:"total"`
}

type taskStats struct {
	Total          int `json:"total"`
	Completed      int `json:"completed"`
	Pending        int `json:"pending"`
	CompletionRate int `json:"completion_rate"`
}

type NoteStats struct {
	Words              int           `json:"words"`
	Sentences          int           `json:"sentences"`
	Characters         int           `json:"characters"`
	TotalCharacters    int           `json:"total_characters"`
	ReadingTimeMinutes int           `json:"reading_time_minutes"`
	Lines              int           `json:"lines"`
	Paragraphs         int           `json:"paragraphs"`
	ListItems          int           `json:"list_items"`
	Tables             int           `json:"tables"`
	Links              int           `json:"links"`
	InternalLinks      int           `json:"internal_links"`
	ExternalLinks      int           `json:"external_links"`
	Wikilinks          int           `json:"wikilinks"`
	CodeBlocks         int           `json:"code_blocks"`
	InlineCode         int           `json:"inline_code"`
	Headings           headingsStats `json:"headings"`
	Tasks              taskStats     `json:"tasks"`
	Images             int           `json:"images"`
	Blockquotes        int           `json:"blockquotes"`
}

type NoteStatsTotals struct {
	TotalNotes          int `json:"total_notes"`
	TotalWords          int `json:"total_words"`
	AverageWordsPerNote int `json:"average_words_per_note"`
	TotalLinks          int `json:"total_links"`
	TotalTasks          int `json:"total_tasks"`
	TotalReadingTime    int `json:"total_reading_time"`
}

type NoteStatsPlugin struct {
	mu           sync.RWMutex
	statsHistory map[string]*NoteStats
}

func NewNoteStatsPlugin() *NoteStatsPlugin {
	return &NoteStatsPlugin{statsHistory: map[string]*NoteStats{}}
}

func (p *NoteStatsPlugin) ID() string {
	return "note_stats"
}

func (p *NoteStatsPlugin) Name() string {
	return "Note Statistics"
}

func (p *NoteStatsPlugin) DefaultEnabled() bool {
	return true
}

func (p *NoteStatsPlugin) Capabilities() []string {
	return []string{"content_analysis"}
}

func (p *NoteStatsPlugin) Analyze(content string) (any, error) {
	return p.calculateStats(content), nil
}

func (p *NoteStatsPlugin) InstallRoutes(r chi.Router, svc *Service) {
	r.Get("/plugins/note_stats/calculate", mango.WrapErrorHandler(func(w http.ResponseWriter, r *http.Request) error {
		content, err := mango.GetQueryParam(r, "content", "", mango.ParseString)
		if err != nil {
			return mango.BadRequestErrorWithCause("invalid query", err)
		}
		stats, enabled, found := svc.AnalyzeContent("note_stats", content)
		if !found {
			return mango.NotFoundError("plugin \"note_stats\" not found")
		}
		if !enabled {
			mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"enabled": false, "stats": nil})
			return nil
		}
		mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"enabled": true, "stats": stats})
		return nil
	}))
}

func (p *NoteStatsPlugin) OnNoteSave(notePath string, content string) (*string, error) {
	stats := p.calculateStats(content)

	p.mu.Lock()
	p.statsHistory[notePath] = stats
	p.mu.Unlock()

	fmt.Printf("Stats %s:\n", notePath)
	fmt.Printf(
		"   %d words | %d sentences | %dm read | %d lines | %d lists | %d tables\n",
		stats.Words,
		stats.Sentences,
		stats.ReadingTimeMinutes,
		stats.Lines,
		stats.ListItems,
		stats.Tables,
	)
	if stats.Links > 0 {
		fmt.Printf("   %d links (%d internal)\n", stats.Links, stats.InternalLinks)
	}
	if stats.Tasks.Total > 0 {
		fmt.Printf("   %d/%d tasks completed\n", stats.Tasks.Completed, stats.Tasks.Total)
	}

	return nil, nil
}

func (p *NoteStatsPlugin) GetStats(notePath string) *NoteStats {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.statsHistory[notePath]
}

func (p *NoteStatsPlugin) GetTotalStats() *NoteStatsTotals {
	p.mu.RLock()
	defer p.mu.RUnlock()

	if len(p.statsHistory) == 0 {
		return nil
	}

	totalWords := 0
	totalLinks := 0
	totalTasks := 0
	totalReadingTime := 0
	for _, stats := range p.statsHistory {
		totalWords += stats.Words
		totalLinks += stats.Links
		totalTasks += stats.Tasks.Total
		totalReadingTime += stats.ReadingTimeMinutes
	}

	totalNotes := len(p.statsHistory)
	average := 0
	if totalNotes > 0 {
		average = int(float64(totalWords)/float64(totalNotes) + 0.5)
	}

	return &NoteStatsTotals{
		TotalNotes:          totalNotes,
		TotalWords:          totalWords,
		AverageWordsPerNote: average,
		TotalLinks:          totalLinks,
		TotalTasks:          totalTasks,
		TotalReadingTime:    totalReadingTime,
	}
}

func (p *NoteStatsPlugin) calculateStats(content string) *NoteStats {
	trimmed := strings.TrimSpace(content)
	words := 0
	if trimmed != "" {
		words = len(regexp.MustCompile(`\S+`).FindAllString(content, -1))
	}

	chars := len(regexp.MustCompile(`\s`).ReplaceAllString(content, ""))
	totalChars := len(content)
	readingTime := int(float64(words)/200.0 + 0.5)
	if readingTime < 1 {
		readingTime = 1
	}
	lines := len(strings.Split(content, "\n"))
	paragraphs := 0
	for _, paragraph := range strings.Split(content, "\n\n") {
		if strings.TrimSpace(paragraph) != "" {
			paragraphs++
		}
	}
	sentences := len(regexp.MustCompile(`[.!?]+(?:\s|$)`).FindAllString(content, -1))
	listItems := countListItems(content)
	tables := len(regexp.MustCompile(`(?m)^\s*\|(?:\s*:?-+:?\s*\|){1,}\s*$`).FindAllString(content, -1))
	markdownLinks := len(regexp.MustCompile(`\[([^\]]+)\]\(([^\)]+)\)`).FindAllString(content, -1))
	markdownInternalLinks := len(regexp.MustCompile(`\[([^\]]+)\]\(([^\)]+\.md)\)`).FindAllString(content, -1))
	wikilinks := len(regexp.MustCompile(`\[\[([^\]|]+)(?:\|[^\]]+)?\]\]`).FindAllString(content, -1))
	links := markdownLinks + wikilinks
	internalLinks := markdownInternalLinks + wikilinks
	codeBlocks := len(regexp.MustCompile("```[\\s\\S]*?```").FindAllString(content, -1))
	inlineCode := len(regexp.MustCompile("`[^`]+`").FindAllString(content, -1))
	h1 := len(regexp.MustCompile(`(?m)^# `).FindAllString(content, -1))
	h2 := len(regexp.MustCompile(`(?m)^## `).FindAllString(content, -1))
	h3 := len(regexp.MustCompile(`(?m)^### `).FindAllString(content, -1))
	totalTasks := len(regexp.MustCompile(`- \[[ x]\]`).FindAllString(content, -1))
	completedTasks := len(regexp.MustCompile(`(?i)- \[x\]`).FindAllString(content, -1))
	pendingTasks := totalTasks - completedTasks
	completionRate := 0
	if totalTasks > 0 {
		completionRate = int((float64(completedTasks)/float64(totalTasks))*100.0 + 0.5)
	}
	images := len(regexp.MustCompile(`!\[([^\]]*)\]\(([^\)]+)\)`).FindAllString(content, -1))
	blockquotes := len(regexp.MustCompile(`(?m)^> `).FindAllString(content, -1))

	return &NoteStats{
		Words:              words,
		Sentences:          sentences,
		Characters:         chars,
		TotalCharacters:    totalChars,
		ReadingTimeMinutes: readingTime,
		Lines:              lines,
		Paragraphs:         paragraphs,
		ListItems:          listItems,
		Tables:             tables,
		Links:              links,
		InternalLinks:      internalLinks,
		ExternalLinks:      links - internalLinks,
		Wikilinks:          wikilinks,
		CodeBlocks:         codeBlocks,
		InlineCode:         inlineCode,
		Headings:           headingsStats{H1: h1, H2: h2, H3: h3, Total: h1 + h2 + h3},
		Tasks: taskStats{
			Total:          totalTasks,
			Completed:      completedTasks,
			Pending:        pendingTasks,
			CompletionRate: completionRate,
		},
		Images:      images,
		Blockquotes: blockquotes,
	}
}

var _ Plugin = (*NoteStatsPlugin)(nil)
var _ ContentAnalyzer = (*NoteStatsPlugin)(nil)
var _ NoteSaveHook = (*NoteStatsPlugin)(nil)
var _ RouteInstaller = (*NoteStatsPlugin)(nil)

func countListItems(content string) int {
	// Go's regexp engine (RE2) does not support lookaheads like (?!\[).
	// Match list prefixes first, then exclude task items in code.
	re := regexp.MustCompile(`^\s*(?:[-*+]|\d+\.)\s+(.*)$`)
	count := 0
	for _, line := range strings.Split(content, "\n") {
		matches := re.FindStringSubmatch(line)
		if len(matches) != 2 {
			continue
		}
		rest := strings.TrimSpace(matches[1])
		if strings.HasPrefix(rest, "[") {
			continue
		}
		count++
	}
	return count
}
