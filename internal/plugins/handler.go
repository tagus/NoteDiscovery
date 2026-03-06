package plugins

import (
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/tagus/mango"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) List(w http.ResponseWriter, _ *http.Request) error {
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"plugins": h.svc.List()})
	return nil
}

func (h *Handler) Toggle(w http.ResponseWriter, r *http.Request) error {
	name := chi.URLParam(r, "plugin_name")
	var body struct {
		Enabled bool `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		return mango.BadRequestErrorWithCause("invalid payload", err)
	}
	if ok := h.svc.Toggle(name, body.Enabled); !ok {
		return mango.NotFoundError("plugin not found")
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"success": true, "plugin": name, "enabled": body.Enabled})
	return nil
}
