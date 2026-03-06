package notes

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"notediscovery/internal/config"
	"notediscovery/internal/plugins"
	"notediscovery/internal/utils/httputil"

	"github.com/go-chi/chi/v5"
	"github.com/tagus/mango"
)

type shareTokenManager interface {
	RevokeByPath(notePath string) (bool, error)
	UpdatePath(oldPath string, newPath string) error
}

type HandlerParams struct {
	Config  *config.Config
	Service *Service
	Plugins *plugins.Service
	Share   shareTokenManager
}

type Handler struct {
	cfg     *config.Config
	svc     *Service
	plugins *plugins.Service
	share   shareTokenManager
}

func NewHandler(params HandlerParams) *Handler {
	return &Handler{cfg: params.Config, svc: params.Service, plugins: params.Plugins, share: params.Share}
}

func (h *Handler) List(w http.ResponseWriter, _ *http.Request) error {
	notes, folders, err := h.svc.ListNotes(true)
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"notes": notes, "folders": folders})
	return nil
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) error {
	notePath := httputil.PathParam(r, "note_path")
	content, err := h.svc.ReadNote(notePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return mango.NotFoundError("note not found")
		}
		return err
	}
	loadPayload := h.plugins.RunHook(plugins.HookOnNoteLoad, map[string]any{"note_path": notePath, "content": content})
	if transformed, ok := loadPayload["content"].(string); ok {
		content = transformed
	}
	metadata, err := h.svc.StatNote(notePath)
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"path": notePath, "content": content, "metadata": metadata})
	return nil
}

func (h *Handler) Upsert(w http.ResponseWriter, r *http.Request) error {
	notePath := httputil.PathParam(r, "note_path")
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
	_, existingErr := h.svc.ReadNote(notePath)
	isNewNote := existingErr != nil

	content := body.Content
	if isNewNote {
		createPayload := h.plugins.RunHook(plugins.HookOnNoteCreate, map[string]any{"note_path": notePath, "initial_content": content})
		if transformed, ok := createPayload["initial_content"].(string); ok {
			content = transformed
		}
	}
	savePayload := h.plugins.RunHook(plugins.HookOnNoteSave, map[string]any{"note_path": notePath, "content": content})
	if transformed, ok := savePayload["content"].(string); ok {
		content = transformed
	}

	_, err := h.svc.WriteNote(notePath, content)
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "path": notePath, "message": "Note saved successfully", "content": content})
	return nil
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) error {
	notePath := httputil.PathParam(r, "note_path")
	if err := h.svc.DeleteNote(notePath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return mango.NotFoundError("note not found")
		}
		return err
	}
	_, _ = h.share.RevokeByPath(notePath)
	h.plugins.RunHook(plugins.HookOnNoteDelete, map[string]any{"note_path": notePath})
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "message": "Note deleted successfully"})
	return nil
}

func (h *Handler) Move(w http.ResponseWriter, r *http.Request) error {
	var body struct{ OldPath, NewPath string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return mango.BadRequestErrorWithCause("invalid payload", err)
	}
	if body.OldPath == "" || body.NewPath == "" {
		return mango.BadRequestError("both oldPath and newPath required")
	}
	if err := h.svc.MoveFile(body.OldPath, body.NewPath, false); err != nil {
		return mango.BadRequestErrorWithCause("failed to move note", err)
	}
	_ = h.share.UpdatePath(body.OldPath, body.NewPath)
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "oldPath": body.OldPath, "newPath": body.NewPath, "message": "Note moved successfully"})
	return nil
}

func (h *Handler) CreateFolder(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return mango.BadRequestErrorWithCause("invalid payload", err)
	}
	if body.Path == "" {
		return mango.BadRequestError("folder path required")
	}
	if err := h.svc.CreateFolder(body.Path); err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "path": body.Path, "message": "Folder created successfully"})
	return nil
}

func (h *Handler) MoveFolder(w http.ResponseWriter, r *http.Request) error {
	var body struct{ OldPath, NewPath string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return mango.BadRequestErrorWithCause("invalid payload", err)
	}
	if err := h.svc.MoveFolder(body.OldPath, body.NewPath); err != nil {
		return mango.BadRequestErrorWithCause("failed to move folder", err)
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "oldPath": body.OldPath, "newPath": body.NewPath, "message": "Folder moved successfully"})
	return nil
}

func (h *Handler) RenameFolder(w http.ResponseWriter, r *http.Request) error {
	return h.MoveFolder(w, r)
}

func (h *Handler) DeleteFolder(w http.ResponseWriter, r *http.Request) error {
	folderPath := httputil.PathParam(r, "folder_path")
	if err := h.svc.DeleteFolder(folderPath); err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "path": folderPath, "message": "Folder deleted successfully"})
	return nil
}

func (h *Handler) GetMedia(w http.ResponseWriter, r *http.Request) error {
	mediaPath := httputil.PathParam(r, "media_path")
	if err := h.svc.ServeMedia(w, r, mediaPath); err != nil {
		return mango.BadRequestErrorWithCause("failed to get media", err)
	}
	return nil
}

func (h *Handler) UploadMedia(w http.ResponseWriter, r *http.Request) error {
	if err := r.ParseMultipartForm(110 << 20); err != nil {
		return mango.BadRequestErrorWithCause("invalid form", err)
	}
	notePath := r.FormValue("note_path")
	file, fileHeader, err := r.FormFile("file")
	if err != nil {
		return mango.BadRequestErrorWithCause("file required", err)
	}
	defer file.Close()
	path, mediaType, err := h.svc.SaveUploadedMedia(notePath, fileHeader, file)
	if err != nil {
		return mango.BadRequestErrorWithCause("failed to save media", err)
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "path": path, "filename": filepath.Base(path), "type": mediaType, "message": fmt.Sprintf("%s uploaded successfully", strings.Title(mediaType))})
	return nil
}

func (h *Handler) MoveMedia(w http.ResponseWriter, r *http.Request) error {
	var body struct{ OldPath, NewPath string }
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return mango.BadRequestErrorWithCause("invalid payload", err)
	}
	if err := h.svc.MoveFile(body.OldPath, body.NewPath, true); err != nil {
		return mango.BadRequestErrorWithCause("failed to move media", err)
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "message": "Media moved successfully", "newPath": body.NewPath})
	return nil
}

func (h *Handler) Search(w http.ResponseWriter, r *http.Request) error {
	if !h.cfg.Search.Enabled {
		return mango.BadRequestError("search is disabled")
	}
	q, err := mango.GetQueryParam(r, "q", "", mango.ParseString)
	if err != nil {
		return mango.BadRequestErrorWithCause("invalid query", err)
	}
	results, err := h.svc.Search(q)
	if err != nil {
		return err
	}
	h.plugins.RunHook(plugins.HookOnSearch, map[string]any{"query": q, "results": results})
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"results": results, "query": q})
	return nil
}

func (h *Handler) ListTags(w http.ResponseWriter, _ *http.Request) error {
	tags, err := h.svc.AllTags()
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"tags": tags})
	return nil
}

func (h *Handler) NotesByTag(w http.ResponseWriter, r *http.Request) error {
	tag := chi.URLParam(r, "tag_name")
	notes, err := h.svc.NotesByTag(tag)
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"tag": tag, "count": len(notes), "notes": notes})
	return nil
}

func (h *Handler) ListTemplates(w http.ResponseWriter, _ *http.Request) error {
	templates, err := h.svc.ListTemplates()
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"templates": templates})
	return nil
}

func (h *Handler) GetTemplate(w http.ResponseWriter, r *http.Request) error {
	name := chi.URLParam(r, "template_name")
	content, err := h.svc.GetTemplateContent(name)
	if err != nil {
		return mango.NotFoundError("template not found")
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"name": name, "content": content})
	return nil
}

func (h *Handler) CreateFromTemplate(w http.ResponseWriter, r *http.Request) error {
	var body struct {
		TemplateName string `json:"templateName"`
		NotePath     string `json:"notePath"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return mango.BadRequestErrorWithCause("invalid payload", err)
	}
	content, err := h.svc.GetTemplateContent(body.TemplateName)
	if err != nil {
		return mango.NotFoundError("template not found")
	}
	final := h.svc.ApplyTemplatePlaceholders(content, body.NotePath)
	createPayload := h.plugins.RunHook(plugins.HookOnNoteCreate, map[string]any{"note_path": body.NotePath, "initial_content": final})
	if transformed, ok := createPayload["initial_content"].(string); ok {
		final = transformed
	}
	savePayload := h.plugins.RunHook(plugins.HookOnNoteSave, map[string]any{"note_path": body.NotePath, "content": final})
	if transformed, ok := savePayload["content"].(string); ok {
		final = transformed
	}
	if _, err := h.svc.WriteNote(body.NotePath, final); err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "path": body.NotePath, "message": "Note created from template successfully", "content": final})
	return nil
}
