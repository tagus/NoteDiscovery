package locales

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
	items, err := h.svc.List()
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, map[string]any{"locales": items})
	return nil
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) error {
	code := chi.URLParam(r, "code")
	item, err := h.svc.Get(code)
	if err != nil {
		return mango.NotFoundError("locale not found")
	}
	mango.WriteJSONResponse(w, http.StatusOK, item)
	return nil
}
