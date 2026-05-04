// Package storage abstrae el almacenamiento binario de los adjuntos.
//
// La interface tiene la mínima superficie necesaria para el MVP. El
// implementador local guarda en el filesystem; uno futuro puede mandar a
// S3/MinIO sin tocar nada más del código.
package storage

import "context"

// Storage guarda, lee y borra binarios identificados por (incidentID, filename).
//
// Save recibe un []byte ya validado y re-encodeado: la capa de storage no
// hace ningún chequeo de seguridad, eso es responsabilidad del caller.
// El storage solo se preocupa por escribir bytes.
//
// Read devuelve os.ErrNotExist si el archivo no existe (típicamente porque
// fue purgado) — el caller decide si es un error o si debe skip.
//
// Delete es idempotente: si el archivo no existe, devuelve nil.
type Storage interface {
	Save(ctx context.Context, incidentID, filename string, data []byte) error
	Read(ctx context.Context, incidentID, filename string) ([]byte, error)
	Delete(ctx context.Context, incidentID, filename string) error
}
