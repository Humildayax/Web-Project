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
	result, err := h.service.GetNews(r.Context())
	if err != nil {
		slog.Error("obteniendo noticias", "err", err)
		writeError(w, http.StatusInternalServerError, "No se pudieron obtener las noticias")
		return
	}
	if result.Stale {
		// Stale = devolvimos cache porque el provider falló. Avisamos por
		// header (cliente puede mostrar banner) y por log (operador alerta).
		w.Header().Set("X-Cache", "stale")
		slog.Warn("noticias servidas desde cache stale (provider falló)")
	}
	writeJSON(w, http.StatusOK, result.Items)
}
