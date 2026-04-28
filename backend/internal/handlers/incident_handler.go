package handlers

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"security-portal/internal/dto"
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

	var req dto.CreateIncidentRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()

	if err := dec.Decode(&req); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeError(w, http.StatusRequestEntityTooLarge, "Cuerpo demasiado grande")
			return
		}
		writeError(w, http.StatusBadRequest, "JSON inválido o con campos desconocidos")
		return
	}

	req.Normalize()

	if err := dto.Validator().Struct(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, dto.FormatValidationError(err))
		return
	}

	incident := req.ToModel()
	// Sin auth aún: registramos el contexto operativo del reporte.
	incident.Metadata = &models.IncidentMetadata{
		SourceIP:   ClientIP(r),
		UserAgent:  r.UserAgent(),
		ReceivedAt: time.Now().UTC(),
	}

	if err := h.service.ProcessNewIncident(r.Context(), incident); err != nil {
		slog.Error("creando incidente", "err", err)
		writeError(w, http.StatusInternalServerError, "Error interno del servidor")
		return
	}

	writeJSON(w, http.StatusCreated, dto.IncidentResponseFromModel(incident))
}

// ClientIP devuelve el IP del cliente para audit logging y rate-limiting.
//
// Confía en este orden:
//  1. X-Real-IP: nuestro nginx lo setea con $remote_addr (sobreescribe lo
//     que mande el cliente), por eso es la fuente más confiable.
//  2. Último valor de X-Forwarded-For: nginx hace `proxy_add_x_forwarded_for`
//     que APPENDEA al header existente; el último elemento es el agregado
//     por el proxy más cercano y no es falsificable. El primero, en cambio,
//     es lo que mandó el cliente y se puede inventar.
//  3. RemoteAddr como último recurso (acceso directo al backend, sin proxy).
func ClientIP(r *http.Request) string {
	if rip := strings.TrimSpace(r.Header.Get("X-Real-IP")); rip != "" {
		return rip
	}
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if i := strings.LastIndex(xff, ","); i >= 0 {
			return strings.TrimSpace(xff[i+1:])
		}
		return strings.TrimSpace(xff)
	}
	return r.RemoteAddr
}
