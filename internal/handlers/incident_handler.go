package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"security-portal/internal/models"
	"security-portal/internal/services"
)

type IncidentHandler struct {
	service      *services.IncidentService
	maxBodyBytes int64
}

func NewIncidentHandler(service *services.IncidentService, maxBodyBytes int64) *IncidentHandler {
	return &IncidentHandler{service: service, maxBodyBytes: maxBodyBytes}
}

func (h *IncidentHandler) CreateIncident(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBodyBytes)

	var incident models.Incident
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&incident); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeError(w, http.StatusRequestEntityTooLarge, "Cuerpo demasiado grande")
			return
		}
		writeError(w, http.StatusBadRequest, "JSON inválido o con campos desconocidos")
		return
	}

	incident.Title = strings.TrimSpace(incident.Title)
	incident.Description = strings.TrimSpace(incident.Description)
	incident.Author = strings.TrimSpace(incident.Author)

	if incident.Title == "" || incident.Description == "" {
		writeError(w, http.StatusBadRequest, "Título y descripción son obligatorios")
		return
	}
	if incident.Author == "" {
		incident.Author = "anonymous"
	}

	// Mientras no haya auth, al menos registramos de dónde vino el reporte.
	incident.Metadata = map[string]any{
		"source_ip":   clientIP(r),
		"user_agent":  r.UserAgent(),
		"received_at": time.Now().UTC().Format(time.RFC3339),
	}

	if err := h.service.ProcessNewIncident(r.Context(), &incident); err != nil {
		slog.Error("creando incidente", "err", err)
		writeError(w, http.StatusInternalServerError, "Error interno del servidor")
		return
	}

	writeJSON(w, http.StatusCreated, incident)
}

func clientIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.Index(xff, ","); i > 0 {
			return strings.TrimSpace(xff[:i])
		}
		return strings.TrimSpace(xff)
	}
	if rip := r.Header.Get("X-Real-IP"); rip != "" {
		return rip
	}
	return r.RemoteAddr
}
