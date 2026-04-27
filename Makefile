.PHONY: help sqlc tidy build run db-up db-down

help:
	@echo "Targets:"
	@echo "  sqlc     - Regenera el código de internal/db a partir de db/queries y scripts/init.sql"
	@echo "  tidy     - go mod tidy"
	@echo "  build    - Compila el binario en ./bin/api"
	@echo "  run      - Ejecuta el servidor (necesita .env configurado y db levantada)"
	@echo "  db-up    - Levanta Postgres con docker compose"
	@echo "  db-down  - Apaga Postgres"

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

db-up:
	docker compose up -d db

db-down:
	docker compose down
