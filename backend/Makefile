.PHONY: help sqlc tidy build run migrate-up migrate-down migrate-status migrate-create

# DSN para los targets migrate-*. Se construye desde las mismas variables
# que usa el binario; alternativamente, exportá DB_DSN en el entorno y se
# usa tal cual.
DB_DSN ?= postgres://$(DB_USER):$(DB_PASSWORD)@$(DB_HOST):$(DB_PORT)/$(DB_NAME)?sslmode=$(DB_SSLMODE)

help:
	@echo "Targets del backend:"
	@echo "  make sqlc             Regenera internal/db a partir de db/queries y migrations/"
	@echo "  make tidy             go mod tidy"
	@echo "  make build            Compila el binario en bin/api"
	@echo "  make run              Ejecuta el servidor (necesita DB levantada: make -C .. dev-db)"
	@echo ""
	@echo "  make migrate-up       Aplica migraciones pendientes (requiere CLI golang-migrate)"
	@echo "  make migrate-down     Reversa la última migración"
	@echo "  make migrate-status   Muestra la versión actual del schema"
	@echo "  make migrate-create name=algo  Crea un par .up.sql/.down.sql nuevo"
	@echo ""
	@echo "Para levantar/apagar la DB y el stack completo, ver Makefile de la raíz."

# Requiere sqlc instalado:
#   go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
sqlc:
	sqlc generate

tidy:
	go mod tidy

build:
	mkdir -p bin
	go build -o bin/api ./cmd/api

run:
	go run ./cmd/api

# ----------------------------------------------------------------------
# Migraciones
#
# El backend ya las corre solo al arrancar (MIGRATE_ON_START=true por
# default). Estos targets son para casos manuales: rollback, generar una
# nueva migración, o aplicar/inspeccionar sin levantar el binario entero.
#
# Requiere la CLI:
#   go install -tags 'pgx5' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
# ----------------------------------------------------------------------

migrate-up:
	migrate -path migrations -database "$(DB_DSN)" up

migrate-down:
	migrate -path migrations -database "$(DB_DSN)" down 1

migrate-status:
	migrate -path migrations -database "$(DB_DSN)" version

migrate-create:
	@test -n "$(name)" || (echo "Uso: make migrate-create name=descripcion_corta" && exit 1)
	migrate create -ext sql -dir migrations -seq $(name)
