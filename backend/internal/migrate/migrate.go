// Package migrate corre las migraciones SQL embebidas en el binario.
//
// Diseño:
//   - Los archivos .sql viven en backend/migrations/ y se embeben con go:embed
//     en el paquete migrations (var FS).
//   - golang-migrate los toma vía source/iofs y aplica los cambios contra
//     Postgres usando el driver pgx/v5 oficial.
//   - Se abre una *sql.DB temporal solo para migrar (el driver de migrate
//     necesita database/sql); se cierra al terminar y la app sigue usando
//     pgxpool. El advisory lock de golang-migrate serializa réplicas: si
//     varias instancias arrancan a la vez, una migra y las demás esperan.
package migrate

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"

	"github.com/golang-migrate/migrate/v4"
	pgxv5 "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	_ "github.com/jackc/pgx/v5/stdlib" // registra el driver "pgx" en database/sql
)

// Up aplica todas las migraciones pendientes contra el DSN dado.
// No-op si el schema ya está al día.
func Up(ctx context.Context, dsn string, source fs.FS) error {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return fmt.Errorf("migrate: abrir conexión: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("migrate: ping: %w", err)
	}

	src, err := iofs.New(source, ".")
	if err != nil {
		return fmt.Errorf("migrate: cargar FS: %w", err)
	}
	defer src.Close()

	dbDriver, err := pgxv5.WithInstance(db, &pgxv5.Config{})
	if err != nil {
		return fmt.Errorf("migrate: instanciar driver pgx/v5: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "pgx5", dbDriver)
	if err != nil {
		return fmt.Errorf("migrate: instanciar runner: %w", err)
	}
	// No m.Close(): cerraría el driver y el *sql.DB; el defer de db.Close()
	// arriba ya se encarga del recurso real.

	switch err := m.Up(); {
	case err == nil:
		// hubo cambios
	case errors.Is(err, migrate.ErrNoChange):
		slog.Info("migrate: nada por aplicar")
	default:
		return fmt.Errorf("migrate: aplicar: %w", err)
	}

	v, dirty, err := m.Version()
	if err != nil && !errors.Is(err, migrate.ErrNilVersion) {
		return fmt.Errorf("migrate: leer versión: %w", err)
	}
	slog.Info("migrate: schema al día", "version", v, "dirty", dirty)
	return nil
}
