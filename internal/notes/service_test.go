package notes

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestService_ParseTags(t *testing.T) {
	svc := NewService(".")

	tests := []struct {
		name     string
		content  string
		expected []string
	}{
		{
			name: "yaml inline list",
			content: "---\n" +
				"tags: [Python, tutorial, backend]\n" +
				"---\n",
			expected: []string{"backend", "python", "tutorial"},
		},
		{
			name: "yaml bullet list",
			content: "---\n" +
				"tags:\n" +
				"  - one\n" +
				"  - Two\n" +
				"---\n",
			expected: []string{"one", "two"},
		},
		{
			name: "inline hash fallback deduped",
			content: "---\n" +
				"title: Demo\n" +
				"---\n" +
				"hello #Tag #tag #other",
			expected: []string{"other", "tag"},
		},
		{
			name:     "no frontmatter",
			content:  "#tag only",
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := svc.ParseTags(tt.content)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestService_WriteNote_NormalizesPath(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(dir)

	actualPath, err := svc.WriteNote("DEV/Test", "content")
	require.NoError(t, err)
	require.Equal(t, "DEV/Test.md", actualPath)

	actualContent, err := svc.ReadNote("DEV/Test")
	require.NoError(t, err)
	require.Equal(t, "content", actualContent)
}

func TestService_IsSecurePath(t *testing.T) {
	dir := t.TempDir()
	svc := NewService(dir)

	require.True(t, svc.IsSecurePath("DEV/Test.md"))
	require.False(t, svc.IsSecurePath("../outside.md"))
}

func TestSanitizeFilename(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "empty", input: "", expected: "unnamed"},
		{name: "dangerous chars", input: `my:file?.md`, expected: "my_file_.md"},
		{name: "strips edges", input: "  _name_  ", expected: "name"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := sanitizeFilename(tt.input)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestService_ApplyTemplatePlaceholders(t *testing.T) {
	svc := NewService(".")
	content := "{{title}}|{{folder}}|{{date}}|{{timestamp}}"

	actual := svc.ApplyTemplatePlaceholders(content, filepath.ToSlash("DEV/test.md"))
	parts := strings.Split(actual, "|")

	require.Len(t, parts, 4)
	require.Equal(t, "test", parts[0])
	require.Equal(t, "DEV", parts[1])
	require.NotEqual(t, "{{date}}", parts[2])
	require.NotEqual(t, "{{timestamp}}", parts[3])
}
