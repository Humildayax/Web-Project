package storage

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LocalStorage guarda los adjuntos en el filesystem local. Los archivos se
// agrupan por incidente: <BasePath>/<incidentID>/<filename>.
//
// Garantías de seguridad de escritura:
//   - incidentID y filename se validan contra path traversal (no `/`, no
//     `..`, solo runas seguras).
//   - El archivo se escribe a un .tmp, fsync, rename atómico. Así un crash
//     a mitad de escritura no deja archivos corruptos.
type LocalStorage struct {
	basePath string
}

func NewLocal(basePath string) (*LocalStorage, error) {
	if basePath == "" {
		return nil, errors.New("storage: basePath vacío")
	}
	if err := os.MkdirAll(basePath, 0o750); err != nil {
		return nil, fmt.Errorf("storage: crear basePath %q: %w", basePath, err)
	}
	return &LocalStorage{basePath: basePath}, nil
}

func (s *LocalStorage) Save(_ context.Context, incidentID, filename string, data []byte) error {
	dir, full, err := s.resolve(incidentID, filename)
	if err != nil {
		return err
	}

	// Cada incidente vive en su propio subdirectorio. 0o750 = dueño rwx,
	// grupo rx, otros nada (el usuario `app` del container es el único que
	// debería leer/escribir).
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return fmt.Errorf("storage: crear dir %q: %w", dir, err)
	}

	tmp := full + ".tmp"
	if err := os.WriteFile(tmp, data, 0o640); err != nil {
		return fmt.Errorf("storage: escribir temp %q: %w", tmp, err)
	}
	if err := os.Rename(tmp, full); err != nil {
		_ = os.Remove(tmp)
		return fmt.Errorf("storage: rename %q -> %q: %w", tmp, full, err)
	}
	return nil
}

func (s *LocalStorage) Read(_ context.Context, incidentID, filename string) ([]byte, error) {
	_, full, err := s.resolve(incidentID, filename)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(full)
	if err != nil {
		// Pasar el error tal cual (incluido os.ErrNotExist) para que el
		// caller pueda usar errors.Is(err, os.ErrNotExist) si quiere.
		return nil, fmt.Errorf("storage: leer %q: %w", full, err)
	}
	return data, nil
}

func (s *LocalStorage) Delete(_ context.Context, incidentID, filename string) error {
	_, full, err := s.resolve(incidentID, filename)
	if err != nil {
		return err
	}
	if err := os.Remove(full); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("storage: borrar %q: %w", full, err)
	}
	return nil
}

// resolve construye el path final y rechaza cualquier intento de path
// traversal. Devuelve (dir del incidente, path completo del archivo).
func (s *LocalStorage) resolve(incidentID, filename string) (string, string, error) {
	if !safeSegment(incidentID) {
		return "", "", fmt.Errorf("storage: incidentID inválido %q", incidentID)
	}
	if !safeSegment(filename) {
		return "", "", fmt.Errorf("storage: filename inválido %q", filename)
	}
	dir := filepath.Join(s.basePath, incidentID)
	full := filepath.Join(dir, filename)

	// Doble defensa: tras construir el path, confirmamos que sigue
	// dentro del base. filepath.Join limpia "..", pero por las dudas.
	absBase, err := filepath.Abs(s.basePath)
	if err != nil {
		return "", "", err
	}
	absFull, err := filepath.Abs(full)
	if err != nil {
		return "", "", err
	}
	if !strings.HasPrefix(absFull, absBase+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("storage: path fuera del base")
	}
	return dir, full, nil
}

// safeSegment acepta solo: a-z A-Z 0-9 . - _.
// Esto cubre UUIDs (con guiones) y nombres tipo "<uuid>.png".
// Rechaza /, \, .., espacios y cualquier carácter de control.
func safeSegment(s string) bool {
	if s == "" || s == "." || s == ".." {
		return false
	}
	for _, r := range s {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '.' || r == '-' || r == '_':
		default:
			return false
		}
	}
	return true
}
