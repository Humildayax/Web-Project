package dto

import (
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/google/uuid"

	"security-portal/internal/models"
)

// AttachmentResponse es la representación pública de un adjunto.
// Solo metadata: el binario nunca viaja en respuestas JSON.
type AttachmentResponse struct {
	ID               uuid.UUID `json:"id"`
	FilenameOriginal string    `json:"filename_original"`
	MimeType         string    `json:"mime_type"`
	SizeBytes        int64     `json:"size_bytes"`
	CreatedAt        time.Time `json:"created_at"`
}

func AttachmentResponseFromModel(a *models.Attachment) AttachmentResponse {
	return AttachmentResponse{
		ID:               a.ID,
		FilenameOriginal: a.FilenameOriginal,
		MimeType:         a.MimeType,
		SizeBytes:        a.SizeBytes,
		CreatedAt:        a.CreatedAt,
	}
}

// SanitizeFilename limpia el filename del cliente para guardarlo como
// metadata legible. NO se usa como path de disco — para eso generamos un
// UUID en el storage.
//
// Reglas:
//   - Conserva solo letras, dígitos, espacios, "." "_" "-".
//   - Reemplaza caracteres no permitidos por "_".
//   - Trunca a 100 runes (suficiente para ser informativo, corto para UI).
//   - Si tras la limpieza queda vacío, devuelve "archivo".
//   - Quita el path si vino con uno (defensa contra "../../foo.png").
func SanitizeFilename(name string) string {
	// filepath.Base elimina cualquier directorio que pudiera venir.
	name = filepath.Base(strings.TrimSpace(name))

	var b strings.Builder
	for _, r := range name {
		switch {
		case unicode.IsLetter(r), unicode.IsDigit(r):
			b.WriteRune(r)
		case r == '.' || r == '_' || r == '-' || r == ' ':
			b.WriteRune(r)
		default:
			b.WriteRune('_')
		}
	}
	clean := strings.TrimSpace(b.String())
	clean = strings.Trim(clean, ".") // evita ".oculto" o ".."
	if clean == "" {
		return "archivo"
	}
	if r := []rune(clean); len(r) > 100 {
		clean = string(r[:100])
	}
	return clean
}
