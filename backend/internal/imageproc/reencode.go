// Package imageproc valida y re-encodea imágenes subidas por usuarios.
//
// El re-encode es una defensa de seguridad — NO un nice-to-have:
//   - Anula payloads polyglot (un archivo válido como JPG y como JS).
//   - Elimina EXIF y metadata (privacidad).
//   - Rechaza imágenes mal formadas que solo un decoder real detectaría.
//   - Rechaza compression bombs validando dimensiones DESPUÉS del decode.
//
// Whitelist de input: image/jpeg, image/png, image/webp.
// Output: JPEG (calidad 85) para inputs JPG/WEBP, PNG para inputs PNG.
// El WEBP no se mantiene como output porque el encoder no está en stdlib.
package imageproc

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"net/http"

	// Side-effect imports: registran los decoders en el paquete image.
	_ "image/jpeg"
	_ "image/png"

	_ "golang.org/x/image/webp"
)

// Errores que el caller puede inspeccionar para devolver mensajes
// específicos al cliente. Solo los devolvemos al usuario en forma genérica;
// el detalle queda en logs server-side.
var (
	ErrUnsupportedFormat = errors.New("formato de imagen no permitido")
	ErrTooLarge          = errors.New("dimensiones de imagen exceden el máximo")
	ErrCorrupted         = errors.New("imagen corrupta o ilegible")
)

// Result devuelve el binario re-encodeado más metadatos derivados.
type Result struct {
	// Data es el binario re-encodeado. Usar este, NO el input.
	Data []byte
	// MimeType del binario re-encodeado ("image/jpeg" o "image/png").
	MimeType string
	// Extension correspondiente (".jpg" o ".png") para el filename de disco.
	Extension string
	// SHA256 del binario re-encodeado en hex.
	SHA256 string
	// Width / Height post-decode, para audit.
	Width  int
	Height int
}

// allowedInputs son los MIME types que aceptamos como entrada (validados por
// magic bytes, no por Content-Type del cliente).
var allowedInputs = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
	"image/webp": true,
}

// Process valida el input y devuelve un binario re-encodeado seguro.
//
// Pasos:
//  1. Detectar MIME por magic bytes (los primeros 512 bytes).
//  2. Verificar contra whitelist.
//  3. Decodificar (si falla, está corrupto o malicioso).
//  4. Validar dimensiones contra maxDim (defensa contra bombas).
//  5. Re-encodear al output canónico.
//  6. Calcular SHA256.
func Process(input []byte, maxDim int) (Result, error) {
	if maxDim <= 0 {
		maxDim = 4096
	}

	mime := http.DetectContentType(input)
	if !allowedInputs[mime] {
		return Result{}, fmt.Errorf("%w: %s", ErrUnsupportedFormat, mime)
	}

	img, format, err := image.Decode(bytes.NewReader(input))
	if err != nil {
		return Result{}, fmt.Errorf("%w: %v", ErrCorrupted, err)
	}

	bounds := img.Bounds()
	w, h := bounds.Dx(), bounds.Dy()
	if w > maxDim || h > maxDim {
		return Result{}, fmt.Errorf("%w: %dx%d (max %d)", ErrTooLarge, w, h, maxDim)
	}

	// El re-encode escribe a un buffer en memoria. Para 5 MB max de input
	// y dimensiones 4096x4096 max, el output también está acotado.
	var buf bytes.Buffer
	var outMime, outExt string

	switch format {
	case "png":
		// PNG preserva transparencia, no tiene sentido recodificar a JPEG.
		if err := png.Encode(&buf, img); err != nil {
			return Result{}, fmt.Errorf("imageproc: encode png: %w", err)
		}
		outMime, outExt = "image/png", ".png"
	case "jpeg", "webp":
		// JPEG calidad 85 es el sweet spot estándar (visualmente sin pérdida
		// perceptible, ~50% más chico que calidad 100).
		if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 85}); err != nil {
			return Result{}, fmt.Errorf("imageproc: encode jpeg: %w", err)
		}
		outMime, outExt = "image/jpeg", ".jpg"
	default:
		return Result{}, fmt.Errorf("%w: decoder devolvió %s", ErrUnsupportedFormat, format)
	}

	out := buf.Bytes()
	sum := sha256.Sum256(out)

	return Result{
		Data:      out,
		MimeType:  outMime,
		Extension: outExt,
		SHA256:    hex.EncodeToString(sum[:]),
		Width:     w,
		Height:    h,
	}, nil
}
