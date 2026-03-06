package plugins

import (
	"regexp"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestNoteStatsPlugin_Analyze(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected *NoteStats
	}{
		{
			name:    "empty content",
			content: "",
			expected: &NoteStats{
				Words:              0,
				Sentences:          0,
				Characters:         0,
				TotalCharacters:    0,
				ReadingTimeMinutes: 1,
				Lines:              1,
				Paragraphs:         0,
				ListItems:          0,
				Tables:             0,
				Links:              0,
				InternalLinks:      0,
				ExternalLinks:      0,
				Wikilinks:          0,
				CodeBlocks:         0,
				InlineCode:         0,
				Headings:           headingsStats{},
				Tasks:              taskStats{},
				Images:             0,
				Blockquotes:        0,
			},
		},
		{
			name: "mixed markdown content",
			content: "alpha beta\n\n" +
				"- item\n" +
				"- [x] task\n" +
				"[md](doc.md)\n" +
				"[[wiki]]\n" +
				"```x```\n" +
				"`i`\n" +
				"# H1\n" +
				"## H2\n" +
				"### H3\n" +
				"> q\n" +
				"![img](a.png)\n",
			expected: &NoteStats{
				Words:              20,
				Sentences:          0,
				Characters:         79,
				TotalCharacters:    100,
				ReadingTimeMinutes: 1,
				Lines:              14,
				Paragraphs:         2,
				ListItems:          1,
				Tables:             0,
				Links:              3,
				InternalLinks:      2,
				ExternalLinks:      1,
				Wikilinks:          1,
				CodeBlocks:         1,
				InlineCode:         2,
				Headings:           headingsStats{H1: 1, H2: 1, H3: 1, Total: 3},
				Tasks:              taskStats{Total: 1, Completed: 1, Pending: 0, CompletionRate: 100},
				Images:             1,
				Blockquotes:        1,
			},
		},
	}

	plugin := NewNoteStatsPlugin()
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actualPayload, err := plugin.Analyze(tt.content)
			require.NoError(t, err)

			actual, ok := actualPayload.(*NoteStats)
			require.True(t, ok)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestNoteStatsPlugin_OnNoteSaveAndTotals(t *testing.T) {
	plugin := NewNoteStatsPlugin()

	_, err := plugin.OnNoteSave("a.md", "one two")
	require.NoError(t, err)
	_, err = plugin.OnNoteSave("b.md", "one two three four")
	require.NoError(t, err)

	actualA := plugin.GetStats("a.md")
	require.NotNil(t, actualA)
	require.Equal(t, 2, actualA.Words)

	totals := plugin.GetTotalStats()
	require.NotNil(t, totals)
	require.Equal(t, 2, totals.TotalNotes)
	require.Equal(t, 6, totals.TotalWords)
	require.Equal(t, 3, totals.AverageWordsPerNote)
}

func TestNoteStatsRegexParityHelpers(t *testing.T) {
	// Guard for future regressions in Go regex behavior used by plugin parity logic.
	content := "- item\n- [ ] task\n"
	listItemMatches := regexp.MustCompile(`^\s*(?:[-*+]|\d+\.)\s+(.*)$`)

	actual := 0
	for _, line := range strings.Split(content, "\n") {
		matches := listItemMatches.FindStringSubmatch(line)
		if len(matches) != 2 {
			continue
		}
		if strings.HasPrefix(strings.TrimSpace(matches[1]), "[") {
			continue
		}
		actual++
	}

	require.Equal(t, 1, actual)
}
