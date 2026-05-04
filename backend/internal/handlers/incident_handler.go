package handlers

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"security-portal/internal/dto"
	"security-portal/internal/imageproc"
	"security-portal/internal/models"
	"security-portal/internal/services"
)

// AttachmentLimits define las cotas que aplica el handler antes de procesar.
// Vienen de config y se inyectan al construirlo.
type AttachmentLimits struct {
	MaxFiles       int   // máximo de archivos por request
	MaxFileBytes   int64 // máximo por archivo (post-magic-byte check)
	MaxImageDim    int   // ancho/alto máximo en pixels (compression bombs)
	MaxMemoryParse int64 // RAM antes de spillar a temp files (multipart)
}

type IncidentHandler struct {
	service      *services.IncidentService
	maxBodyBytes int64
	limits       AttachmentLimits
}

func NewIncidentHandler(service *services.IncidentService, maxBodyBytes int64, limits AttachmentLimits) *IncidentHandler {
	return &IncidentHandler{
		service:      service,
		maxBodyBytes: maxBodyBytes,
		limits:       limits,
	}
}

func (h *IncidentHandler) CreateIncident(w http.ResponseWriter, r *http.Request) {
	// MaxBytesReader es la primera barrera: si el body total supera el cap,
	// la lectura corta y devolvemos 413. Esto cubre el caso de un atacante
	// que mande un GB en el body sin importar los Content-Length headers.
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBodyBytes)

	if !strings.HasPrefix(r.Header.Get("Content-Type"), "multipart/form-data") {
		writeError(w, http.StatusUnsupportedMediaType,
			"el endpoint requiere multipart/form-data")
		return
	}

	if err := r.ParseMultipartForm(h.limits.MaxMemoryParse); err != nil {
		var mbe *http.MaxBytesError
		if errors.As(err, &mbe) {
			writeError(w, http.StatusRequestEntityTooLarge, "Cuerpo demasiado grande")
			return
		}
		writeError(w, http.StatusBadRequest, "multipart inválido")
		return
	}
	// Limpia los temp files que ParseMultipartForm pueda haber creado.
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()

	req := dto.CreateIncidentRequest{
		Title:       r.FormValue("title"),
		Description: r.FormValue("description"),
		Author:      r.FormValue("author"),
	}
	req.Normalize()

	if err := dto.Validator().Struct(&req); err != nil {
		writeError(w, http.StatusUnprocessableEntity, dto.FormatValidationError(err))
		return
	}

	files := r.MultipartForm.File["attachments"]
	if len(files) > h.limits.MaxFiles {
		writeError(w, http.StatusUnprocessableEntity,
			fmt.Sprintf("máximo %d adjuntos por reporte", h.limits.MaxFiles))
		return
	}

	processed, errMsg := h.processFiles(files)
	if errMsg != "" {
		writeError(w, http.StatusUnprocessableEntity, errMsg)
		return
	}

	incident := req.ToModel()
	incident.Metadata = &models.IncidentMetadata{
		SourceIP:   ClientIP(r),
		UserAgent:  r.UserAgent(),
		ReceivedAt: time.Now().UTC(),
	}

	saved, err := h.service.ProcessNewIncident(r.Context(), incident, processed)
	if err != nil {
		slog.Error("creando incidente", "err", err)
		writeError(w, http.StatusInternalServerError, "Error interno del servidor")
		return
	}

	writeJSON(w, http.StatusCreated, dto.IncidentResponseFromModel(incident, saved))
}

// processFiles valida cada archivo subido y lo re-encodea. Si CUALQUIERA
// falla, devuelve un error legible y no se procesa nada — todo o nada.
//
// Mensajes al cliente son genéricos a propósito: detalles del fallo
// (magic bytes wrong, dimensiones, formato exacto) podrían ayudar a un
// atacante a evadir validaciones. El detalle queda en logs.
func (h *IncidentHandler) processFiles(files []*multipart.FileHeader) ([]services.ProcessedAttachment, string) {
	out := make([]services.ProcessedAttachment, 0, len(files))

	for i, fh := range files {
		// Tamaño declarado en el header. Si miente, el LimitReader corta.
		if fh.Size > h.limits.MaxFileBytes {
			slog.Warn("attachment: size header excede límite",
				"index", i, "size", fh.Size, "max", h.limits.MaxFileBytes, "name", fh.Filename)
			return nil, fmt.Sprintf("archivo #%d excede el tamaño máximo permitido", i+1)
		}

		data, err := readMultipartFile(fh, h.limits.MaxFileBytes)
		if err != nil {
			slog.Warn("attachment: lectura falló",
				"index", i, "err", err, "name", fh.Filename)
			return nil, fmt.Sprintf("archivo #%d no pudo leerse", i+1)
		}

		result, err := imageproc.Process(data, h.limits.MaxImageDim)
		if err != nil {
			// El detalle del error queda en logs (formato exacto, dim, etc.).
			slog.Warn("attachment: imageproc rechazó",
				"index", i, "err", err, "name", fh.Filename, "size", len(data))
			return nil, fmt.Sprintf("archivo #%d no es una imagen válida (jpg/png/webp, máx %dpx)",
				i+1, h.limits.MaxImageDim)
		}

		out = append(out, services.ProcessedAttachment{
			FilenameOriginal: dto.SanitizeFilename(fh.Filename),
			Data:             result.Data,
			MimeType:         result.MimeType,
			Extension:        result.Extension,
			SHA256:           result.SHA256,
		})
	}
	return out, ""
}

// readMultipartFile lee el FileHeader hasta `max+1` bytes. Si el archivo
// supera `max`, devolvemos error en vez de silenciosamente truncar.
func readMultipartFile(fh *multipart.FileHeader, max int64) ([]byte, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// LimitReader devuelve EOF al alcanzar max+1; si llegamos a leer max+1
	// significa que el archivo era más grande.
	limited := io.LimitReader(f, max+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > max {
		return nil, fmt.Errorf("archivo excede %d bytes", max)
	}
	return data, nil
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
