package server

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"notediscovery/internal/plugins"
	"notediscovery/internal/share"

	"github.com/go-chi/chi/v5"
	"github.com/tagus/mango"
)

type Handlers struct {
	params Params
}

func (h *Handlers) Health(w http.ResponseWriter, _ *http.Request) error {
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"status": "healthy", "app": h.params.Config.App.Name, "version": h.params.Version})
	return nil
}

func (h *Handlers) ServiceWorker(w http.ResponseWriter, _ *http.Request) error {
	buf, err := os.ReadFile(filepath.Join(h.params.StaticDir, "sw.js"))
	if err != nil {
		return mango.NotFoundError("service worker not found")
	}
	content := strings.ReplaceAll(string(buf), "__APP_VERSION__", h.params.Version)
	w.Header().Set("Content-Type", "application/javascript")
	_, _ = w.Write([]byte(content))
	return nil
}

func (h *Handlers) LoginPage(w http.ResponseWriter, r *http.Request) error {
	if !h.params.Auth.Enabled() {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return nil
	}
	if cookie, _ := r.Cookie("nd_auth"); cookie != nil && h.params.Auth.IsAuthenticated(cookie.Value) {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return nil
	}
	buf, err := os.ReadFile(filepath.Join(h.params.StaticDir, "login.html"))
	if err != nil {
		return err
	}
	content := strings.ReplaceAll(string(buf), "NoteDiscovery", h.params.Config.App.Name)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(content))
	return nil
}

func (h *Handlers) Login(w http.ResponseWriter, r *http.Request) error {
	if !h.params.Auth.Enabled() {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return nil
	}
	if err := r.ParseForm(); err != nil {
		return mango.BadRequestErrorWithCause("invalid form", err)
	}
	password := r.Form.Get("password")
	if !h.params.Auth.VerifyPassword(password) {
		http.Redirect(w, r, "/login?error=incorrect_password", http.StatusSeeOther)
		return nil
	}
	http.SetCookie(w, &http.Cookie{Name: "nd_auth", Value: h.params.Auth.NewCookieValue(), Path: "/", MaxAge: h.params.Auth.MaxAge(), HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
	return nil
}

func (h *Handlers) Logout(w http.ResponseWriter, r *http.Request) error {
	http.SetCookie(w, &http.Cookie{Name: "nd_auth", Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteLaxMode})
	http.Redirect(w, r, "/login", http.StatusSeeOther)
	return nil
}

func (h *Handlers) GetConfig(w http.ResponseWriter, _ *http.Request) error {
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{
		"name":           h.params.Config.App.Name,
		"version":        h.params.Version,
		"searchEnabled":  h.params.Config.Search.Enabled,
		"demoMode":       strings.EqualFold(os.Getenv("DEMO_MODE"), "true"),
		"alreadyDonated": strings.EqualFold(os.Getenv("ALREADY_DONATED"), "true"),
		"authentication": map[string]any{"enabled": h.params.Config.Authentication.Enabled},
	})
	return nil
}

func (h *Handlers) ListThemes(w http.ResponseWriter, _ *http.Request) error {
	themes, err := h.params.Themes.List()
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"themes": themes})
	return nil
}

func (h *Handlers) GetTheme(w http.ResponseWriter, r *http.Request) error {
	themeID := chi.URLParam(r, "theme_id")
	css, err := h.params.Themes.GetCSS(themeID)
	if err != nil {
		return mango.NotFoundError("theme not found")
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"css": css, "theme_id": themeID})
	return nil
}

func (h *Handlers) ListLocales(w http.ResponseWriter, _ *http.Request) error {
	items, err := h.params.Locales.List()
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"locales": items})
	return nil
}

func (h *Handlers) GetLocale(w http.ResponseWriter, r *http.Request) error {
	code := chi.URLParam(r, "code")
	item, err := h.params.Locales.Get(code)
	if err != nil {
		return mango.NotFoundError("locale not found")
	}
	mango.WriteJSONResponse(w, http.StatusOK, item)
	return nil
}

func (h *Handlers) ListNotes(w http.ResponseWriter, _ *http.Request) error {
	notes, folders, err := h.params.Notes.ListNotes(true)
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"notes": notes, "folders": folders})
	return nil
}

func (h *Handlers) GetNote(w http.ResponseWriter, r *http.Request) error {
	notePath := chi.URLParam(r, "note_path")
	content, err := h.params.Notes.ReadNote(notePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return mango.NotFoundError("note not found")
		}
		return err
	}
	loadPayload := h.params.Plugins.RunHook(plugins.HookOnNoteLoad, map[string]any{
		"note_path": notePath,
		"content":   content,
	})
	if transformed, ok := loadPayload["content"].(string); ok {
		content = transformed
	}

	metadata, err := h.params.Notes.StatNote(notePath)
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"path": notePath, "content": content, "metadata": metadata})
	return nil
}

func (h *Handlers) UpsertNote(w http.ResponseWriter, r *http.Request) error {
	notePath := chi.URLParam(r, "note_path")
	var body struct {
		Content string `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		if errors.Is(err, io.EOF) {
			body.Content = ""
		} else {
			return mango.BadRequestErrorWithCause("invalid payload", err)
		}
	}
	_, existingErr := h.params.Notes.ReadNote(notePath)
	isNewNote := existingErr != nil

	content := body.Content
	if isNewNote {
		createPayload := h.params.Plugins.RunHook(plugins.HookOnNoteCreate, map[string]any{
			"note_path":       notePath,
			"initial_content": content,
		})
		if transformed, ok := createPayload["initial_content"].(string); ok {
			content = transformed
		}
	}
	savePayload := h.params.Plugins.RunHook(plugins.HookOnNoteSave, map[string]any{
		"note_path": notePath,
		"content":   content,
	})
	if transformed, ok := savePayload["content"].(string); ok {
		content = transformed
	}

	_, err := h.params.Notes.WriteNote(notePath, content)
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "path": notePath, "message": "Note saved successfully", "content": content})
	return nil
}

func (h *Handlers) DeleteNote(w http.ResponseWriter, r *http.Request) error {
	notePath := chi.URLParam(r, "note_path")
	if err := h.params.Notes.DeleteNote(notePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return mango.NotFoundError("note not found")
		}
		return err
	}
	_, _ = h.params.Share.RevokeByPath(notePath)
	h.params.Plugins.RunHook(plugins.HookOnNoteDelete, map[string]any{"note_path": notePath})
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "message": "Note deleted successfully"})
	return nil
}

func (h *Handlers) MoveNote(w http.ResponseWriter, r *http.Request) error {
	var body struct{ OldPath, NewPath string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return mango.BadRequestErrorWithCause("invalid payload", err)
	}
	if body.OldPath == "" || body.NewPath == "" {
		return mango.BadRequestError("both oldPath and newPath required")
	}
	if err := h.params.Notes.MoveFile(body.OldPath, body.NewPath, false); err != nil {
		return mango.BadRequestErrorWithCause("failed to move note", err)
	}
	_ = h.params.Share.UpdatePath(body.OldPath, body.NewPath)
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "oldPath": body.OldPath, "newPath": body.NewPath, "message": "Note moved successfully"})
	return nil
}

func (h *Handlers) CreateFolder(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return mango.BadRequestErrorWithCause("invalid payload", err)
	}
	if body.Path == "" {
		return mango.BadRequestError("folder path required")
	}
	if err := h.params.Notes.CreateFolder(body.Path); err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "path": body.Path, "message": "Folder created successfully"})
	return nil
}

func (h *Handlers) MoveFolder(w http.ResponseWriter, r *http.Request) error {
	var body struct{ OldPath, NewPath string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return mango.BadRequestErrorWithCause("invalid payload", err)
	}
	if err := h.params.Notes.MoveFolder(body.OldPath, body.NewPath); err != nil {
		return mango.BadRequestErrorWithCause("failed to move folder", err)
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "oldPath": body.OldPath, "newPath": body.NewPath, "message": "Folder moved successfully"})
	return nil
}

func (h *Handlers) RenameFolder(w http.ResponseWriter, r *http.Request) error {
	return h.MoveFolder(w, r)
}

func (h *Handlers) DeleteFolder(w http.ResponseWriter, r *http.Request) error {
	folderPath := chi.URLParam(r, "folder_path")
	if err := h.params.Notes.DeleteFolder(folderPath); err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "path": folderPath, "message": "Folder deleted successfully"})
	return nil
}

func (h *Handlers) GetMedia(w http.ResponseWriter, r *http.Request) error {
	mediaPath := chi.URLParam(r, "media_path")
	if err := h.params.Notes.ServeMedia(w, r, mediaPath); err != nil {
		return mango.BadRequestErrorWithCause("failed to get media", err)
	}
	return nil
}

func (h *Handlers) UploadMedia(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseMultipartForm(110 << 20); err != nil {
		return mango.BadRequestErrorWithCause("invalid form", err)
	}
	notePath := r.FormValue("note_path")
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		return mango.BadRequestErrorWithCause("file required", err)
	}
	defer file.Close()
	path, mediaType, err := h.params.Notes.SaveUploadedMedia(notePath, fileHeader, file)
	if err != nil {
		return mango.BadRequestErrorWithCause("failed to save media", err)
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "path": path, "filename": filepath.Base(path), "type": mediaType, "message": fmt.Sprintf("%s uploaded successfully", strings.Title(mediaType))})
	return nil
}

func (h *Handlers) MoveMedia(w http.ResponseWriter, r *http.Request) error {
	var body struct{ OldPath, NewPath string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return mango.BadRequestErrorWithCause("invalid payload", err)
	}
	if err := h.params.Notes.MoveFile(body.OldPath, body.NewPath, true); err != nil {
		return mango.BadRequestErrorWithCause("failed to move media", err)
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "message": "Media moved successfully", "newPath": body.NewPath})
	return nil
}

func (h *Handlers) Search(w http.ResponseWriter, r *http.Request) error {
	if !h.params.Config.Search.Enabled {
		return mango.BadRequestError("search is disabled")
	}
	q, err := mango.GetQueryParam(r, "q", "", mango.ParseString)
	if err != nil {
		return mango.BadRequestErrorWithCause("invalid query", err)
	}
	results, err := h.params.Notes.Search(q)
	if err != nil {
		return err
	}
	h.params.Plugins.RunHook(plugins.HookOnSearch, map[string]any{
		"query":   q,
		"results": results,
	})
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"results": results, "query": q})
	return nil
}

func (h *Handlers) Graph(w http.ResponseWriter, _ *http.Request) error {
	data, err := h.params.Graph.Build()
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, data)
	return nil
}

func (h *Handlers) ListTags(w http.ResponseWriter, _ *http.Request) error {
	tags, err := h.params.Notes.AllTags()
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"tags": tags})
	return nil
}

func (h *Handlers) NotesByTag(w http.ResponseWriter, r *http.Request) error {
	tag := chi.URLParam(r, "tag_name")
	notes, err := h.params.Notes.NotesByTag(tag)
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"tag": tag, "count": len(notes), "notes": notes})
	return nil
}

func (h *Handlers) ListTemplates(w http.ResponseWriter, _ *http.Request) error {
	templates, err := h.params.Notes.ListTemplates()
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"templates": templates})
	return nil
}

func (h *Handlers) GetTemplate(w http.ResponseWriter, r *http.Request) error {
	name := chi.URLParam(r, "template_name")
	content, err := h.params.Notes.GetTemplateContent(name)
	if err != nil {
		return mango.NotFoundError("template not found")
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"name": name, "content": content})
	return nil
}

func (h *Handlers) CreateFromTemplate(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		TemplateName string `json:"templateName"`
		NotePath     string `json:"notePath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return mango.BadRequestErrorWithCause("invalid payload", err)
	}
	content, err := h.params.Notes.GetTemplateContent(body.TemplateName)
	if err != nil {
		return mango.NotFoundError("template not found")
	}
	final := h.params.Notes.ApplyTemplatePlaceholders(content, body.NotePath)
	createPayload := h.params.Plugins.RunHook(plugins.HookOnNoteCreate, map[string]any{
		"note_path":       body.NotePath,
		"initial_content": final,
	})
	if transformed, ok := createPayload["initial_content"].(string); ok {
		final = transformed
	}
	savePayload := h.params.Plugins.RunHook(plugins.HookOnNoteSave, map[string]any{
		"note_path": body.NotePath,
		"content":   final,
	})
	if transformed, ok := savePayload["content"].(string); ok {
		final = transformed
	}

	if _, err := h.params.Notes.WriteNote(body.NotePath, final); err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "path": body.NotePath, "message": "Note created from template successfully", "content": final})
	return nil
}

func (h *Handlers) ListPlugins(w http.ResponseWriter, _ *http.Request) error {
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"plugins": h.params.Plugins.List()})
	return nil
}

func (h *Handlers) CalculateNoteStats(w http.ResponseWriter, r *http.Request) error {
	content, err := mango.GetQueryParam(r, "content", "", mango.ParseString)
	if err != nil {
		return mango.BadRequestErrorWithCause("invalid query", err)
	}
	stats, enabled, found := h.params.Plugins.AnalyzeContent("note_stats", content)
	if !found {
		return mango.NotFoundError("plugin \"note_stats\" not found")
	}
	if !enabled {
		mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"enabled": false, "stats": nil})
		return nil
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"enabled": true, "stats": stats})
	return nil
}

func (h *Handlers) TogglePlugin(w http.ResponseWriter, r *http.Request) error {
	name := chi.URLParam(r, "plugin_name")
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return mango.BadRequestErrorWithCause("invalid payload", err)
	}
	if ok := h.params.Plugins.Toggle(name, body.Enabled); !ok {
		return mango.NotFoundError("plugin not found")
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "plugin": name, "enabled": body.Enabled})
	return nil
}

func (h *Handlers) CreateShare(w http.ResponseWriter, r *http.Request) error {
	notePath := chi.URLParam(r, "note_path")
	if !strings.HasSuffix(strings.ToLower(notePath), ".md") {
		notePath += ".md"
	}
	if _, err := h.params.Notes.ReadNote(notePath); err != nil {
		return mango.NotFoundError("note not found")
	}
	theme := "light"
	if r.ContentLength > 0 {
		var body struct {
			Theme string `json:"theme"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err == nil && body.Theme != "" {
			theme = body.Theme
		}
	}
	token, err := h.params.Share.CreateToken(notePath, theme)
	if err != nil {
		return err
	}
	base := fmt.Sprintf("%s://%s", schemeFor(r), r.Host)
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "token": token, "url": base + "/share/" + token, "path": notePath, "theme": theme})
	return nil
}

func (h *Handlers) GetShareStatus(w http.ResponseWriter, r *http.Request) error {
	notePath := chi.URLParam(r, "note_path")
	if !strings.HasSuffix(strings.ToLower(notePath), ".md") {
		notePath += ".md"
	}
	info, err := h.params.Share.InfoForPath(notePath)
	if err != nil {
		return err
	}
	if shared, _ := info["shared"].(bool); shared {
		base := fmt.Sprintf("%s://%s", schemeFor(r), r.Host)
		info["url"] = base + "/share/" + info["token"].(string)
	}
	mango.WriteJSONResponse(w, http.StatusOK, info)
	return nil
}

func (h *Handlers) ListSharedNotes(w http.ResponseWriter, _ *http.Request) error {
	paths, err := h.params.Share.Paths()
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"paths": paths})
	return nil
}

func (h *Handlers) DeleteShare(w http.ResponseWriter, r *http.Request) error {
	notePath := chi.URLParam(r, "note_path")
	if !strings.HasSuffix(strings.ToLower(notePath), ".md") {
		notePath += ".md"
	}
	success, err := h.params.Share.RevokeByPath(notePath)
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": success, "message": ternary(success, "Share revoked", "Note was not shared")})
	return nil
}

func (h *Handlers) ViewSharedNote(w http.ResponseWriter, r *http.Request) error {
	token := chi.URLParam(r, "token")
	info, err := h.params.Share.NoteByToken(token)
	if err != nil {
		return mango.NotFoundError("shared note not found")
	}
	content, err := h.params.Notes.ReadNote(info.Path)
	if err != nil {
		return mango.NotFoundError("note not found")
	}
	title := strings.TrimSuffix(filepath.Base(info.Path), filepath.Ext(info.Path))
	html := h.params.Share.RenderSharedHTML(title, share.StripFrontmatter(content), info.Theme)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
	return nil
}

func (h *Handlers) CatchAll(w http.ResponseWriter, _ *http.Request) error {
	buf, err := os.ReadFile(filepath.Join(h.params.StaticDir, "index.html"))
	if err != nil {
		return err
	}
	content := strings.ReplaceAll(string(buf), "<title>NoteDiscovery</title>", "<title>"+h.params.Config.App.Name+"</title>")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(content))
	return nil
}

func schemeFor(r *http.Request) string {
	if r.Header.Get("X-Forwarded-Proto") == "https" || r.TLS != nil {
		return "https"
	}
	return "http"
}

func ternary[T any](cond bool, a T, b T) T {
	if cond {
		return a
	}
	return b
}
