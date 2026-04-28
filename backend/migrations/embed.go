// Package migrations embebe los archivos .sql de migraciones en el binario.
// El runner que las consume vive en internal/migrate.
package migrations

import "embed"

// FS contiene todas las migraciones empaquetadas en el binario.
// Naming convention: NNNNN_descripcion.up.sql / NNNNN_descripcion.down.sql.
//
//go:embed *.sql
var FS embed.FS
