package handlers

import (
	"log/slog"
	"net/http"

	"security-portal/internal/services"
)

type NewsHandler struct {
	service *services.NewsService
}

func NewNewsHandler(service *services.NewsService) *NewsHandler {
	return &NewsHandler{service: service}
}

func (h *NewsHandler) GetNews(w http.ResponseWriter, r *http.Request) {
	news, err := h.service.GetNews(r.Context())
	if err != nil {
		slog.Error("obteniendo noticias", "err", err)
		writeError(w, http.StatusInternalServerError, "No se pudieron obtener las noticias")
		return
	}
	writeJSON(w, http.StatusOK, news)
}
