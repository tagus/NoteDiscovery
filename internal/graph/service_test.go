package graph

import (
	"testing"

	"notediscovery/internal/notes"

	"github.com/stretchr/testify/require"
)

func TestService_BuildFiltersExternalLinksAndBuildsInternalEdges(t *testing.T) {
	dir := t.TempDir()
	notesSvc := notes.NewService(dir)

	_, err := notesSvc.WriteNote("DEV/source.md", "[[target]]\n[Internal](target.md)\n[External](https://example.com)\n[Mail](mailto:test@example.com)\n[Anchor](#x)\n[Data](data:text/plain,x)")
	require.NoError(t, err)
	_, err = notesSvc.WriteNote("DEV/target.md", "hello")
	require.NoError(t, err)

	svc := NewService(notesSvc)
	actual, err := svc.Build()
	require.NoError(t, err)

	edgesAny, ok := actual["edges"].([]map[string]any)
	require.True(t, ok)
	require.Len(t, edgesAny, 1)

	require.Equal(t, "DEV/source.md", edgesAny[0]["source"])
	require.Equal(t, "DEV/target.md", edgesAny[0]["target"])
}
