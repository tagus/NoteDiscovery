package plugins

import (
	"testing"

	"github.com/stretchr/testify/require"
)

type mockPlugin struct {
	enabled bool
}

func (m *mockPlugin) ID() string             { return "mock" }
func (m *mockPlugin) Name() string           { return "Mock" }
func (m *mockPlugin) DefaultEnabled() bool   { return m.enabled }
func (m *mockPlugin) Capabilities() []string { return []string{} }
func (m *mockPlugin) OnNoteCreate(_ string, initialContent string) (string, error) {
	return initialContent + "-created", nil
}
func (m *mockPlugin) OnNoteSave(_ string, content string) (*string, error) {
	actual := content + "-saved"
	return &actual, nil
}

var _ Plugin = (*mockPlugin)(nil)
var _ NoteCreateHook = (*mockPlugin)(nil)
var _ NoteSaveHook = (*mockPlugin)(nil)

func TestService_RunHook_CreateAndSave(t *testing.T) {
	svc := &Service{dir: t.TempDir(), plugins: map[string]*runtimePlugin{}}
	svc.Register(&mockPlugin{enabled: true})

	actualCreate := svc.RunHook(HookOnNoteCreate, map[string]any{"note_path": "a.md", "initial_content": "x"})
	require.Equal(t, "x-created", actualCreate["initial_content"])

	actualSave := svc.RunHook(HookOnNoteSave, map[string]any{"note_path": "a.md", "content": "x"})
	require.Equal(t, "x-saved", actualSave["content"])
}

func TestService_Toggle(t *testing.T) {
	svc := &Service{dir: t.TempDir(), plugins: map[string]*runtimePlugin{}}
	svc.Register(&mockPlugin{enabled: true})

	require.True(t, svc.Enabled("mock"))
	require.True(t, svc.Toggle("mock", false))
	require.False(t, svc.Enabled("mock"))
	require.False(t, svc.Toggle("missing", true))
}
