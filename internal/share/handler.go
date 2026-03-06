package share

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"strings"

	"notediscovery/internal/notes"
	"notediscovery/internal/utils/httputil"

	"github.com/go-chi/chi/v5"
	"github.com/tagus/mango"
)

type Handler struct {
	service *Service
	notes   *notes.Service
}

func NewHandler(service *Service, notes *notes.Service) *Handler {
	return &Handler{service: service, notes: notes}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) error {
	notePath := httputil.PathParam(r, "note_path")
	if !strings.HasSuffix(strings.ToLower(notePath), ".md") {
		notePath += ".md"
	}
	if _, err := h.notes.ReadNote(notePath); err != nil {
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
	token, err := h.service.CreateToken(notePath, theme)
	if err != nil {
		return err
	}
	base := fmt.Sprintf("%s://%s", httputil.SchemeFor(r), r.Host)
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "token": token, "url": base + "/share/" + token, "path": notePath, "theme": theme})
	return nil
}

func (h *Handler) Status(w http.ResponseWriter, r *http.Request) error {
	notePath := httputil.PathParam(r, "note_path")
	if !strings.HasSuffix(strings.ToLower(notePath), ".md") {
		notePath += ".md"
	}
	info, err := h.service.InfoForPath(notePath)
	if err != nil {
		return err
	}
	if shared, _ := info["shared"].(bool); shared {
		base := fmt.Sprintf("%s://%s", httputil.SchemeFor(r), r.Host)
		info["url"] = base + "/share/" + info["token"].(string)
	}
	mango.WriteJSONResponse(w, http.StatusOK, info)
	return nil
}

func (h *Handler) List(w http.ResponseWriter, _ *http.Request) error {
	paths, err := h.service.Paths()
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"paths": paths})
	return nil
}

func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) error {
	notePath := httputil.PathParam(r, "note_path")
	if !strings.HasSuffix(strings.ToLower(notePath), ".md") {
		notePath += ".md"
	}
	success, err := h.service.RevokeByPath(notePath)
	if err != nil {
		return err
	}
	message := "Note was not shared"
	if success {
		message = "Share revoked"
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": success, "message": message})
	return nil
}

func (h *Handler) View(w http.ResponseWriter, r *http.Request) error {
	token := chi.URLParam(r, "token")
	info, err := h.service.NoteByToken(token)
	if err != nil {
		return mango.NotFoundError("shared note not found")
	}
	content, err := h.notes.ReadNote(info.Path)
	if err != nil {
		return mango.NotFoundError("note not found")
	}
	title := strings.TrimSuffix(filepath.Base(info.Path), filepath.Ext(info.Path))
	html := h.service.RenderSharedHTML(title, StripFrontmatter(content), info.Theme)
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(html))
	return nil
}
