package graph

import (
	"net/http"

	"github.com/tagus/mango"
)

type Handler struct {
	svc *Service
}

func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) Get(w http.ResponseWriter, _ *http.Request) error {
	data, err := h.svc.Build()
	if err != nil {
		return err
	}
	mango.WriteJSONResponse(w, http.StatusOK, data)
	return nil
}
