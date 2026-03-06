package themes

import (
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
	themes, err := h.svc.List()
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"themes": themes})
	return nil
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) error {
	themeID := chi.URLParam(r, "theme_id")
	css, err := h.svc.GetCSS(themeID)
	if err != nil {
		return mango.NotFoundError("theme not found")
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"css": css, "theme_id": themeID})
	return nil
}
