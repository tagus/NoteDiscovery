package share

import (
	"os"
	"path/filepath"
	"testing"

	"notediscovery/internal/notes"

	"github.com/stretchr/testify/require"
)

func TestService_TokenLifecycle(t *testing.T) {
	dir := t.TempDir()
	notesSvc := notes.NewService(dir)
	svc := NewService(notesSvc, dir)

	tokenA, err := svc.CreateToken("a.md", "light")
	require.NoError(t, err)
	require.NotEmpty(t, tokenA)

	// Python parity: create on existing note path should return existing token.
	tokenA2, err := svc.CreateToken("a.md", "dark")
	require.NoError(t, err)
	require.Equal(t, tokenA, tokenA2)

	actualInfo, err := svc.InfoForPath("a.md")
	require.NoError(t, err)
	require.Equal(t, true, actualInfo["shared"])
	require.Equal(t, tokenA, actualInfo["token"])

	paths, err := svc.Paths()
	require.NoError(t, err)
	require.Equal(t, []string{"a.md"}, paths)

	noteInfo, err := svc.NoteByToken(tokenA)
	require.NoError(t, err)
	require.Equal(t, "a.md", noteInfo.Path)

	require.NoError(t, svc.UpdatePath("a.md", "DEV/a.md"))
	actualInfoMoved, err := svc.InfoForPath("DEV/a.md")
	require.NoError(t, err)
	require.Equal(t, true, actualInfoMoved["shared"])

	actualRevoked, err := svc.RevokeByPath("DEV/a.md")
	require.NoError(t, err)
	require.True(t, actualRevoked)

	actualInfoRevoked, err := svc.InfoForPath("DEV/a.md")
	require.NoError(t, err)
	require.Equal(t, false, actualInfoRevoked["shared"])
}

func TestStripFrontmatter(t *testing.T) {
	tests := []struct {
		name     string
		content  string
		expected string
	}{
		{name: "with frontmatter", content: "---\ntitle: x\n---\n# body", expected: "# body"},
		{name: "without frontmatter", content: "# body", expected: "# body"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := StripFrontmatter(tt.content)
			require.Equal(t, tt.expected, actual)
		})
	}
}

func TestRenderSharedHTML_LoadsThemeFallback(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "light.css"), []byte("body{color:black;}"), 0o644))
	notesSvc := notes.NewService(t.TempDir())
	svc := NewService(notesSvc, dir)

	actual := svc.RenderSharedHTML("Title", "<script>", "missing")
	require.Contains(t, actual, "body{color:black;}")
	require.Contains(t, actual, "&lt;script&gt;")
}
