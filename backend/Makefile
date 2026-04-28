.PHONY: help sqlc tidy build run

help:
	@echo "Targets del backend:"
	@echo "  make sqlc    Regenera internal/db a partir de db/queries y scripts/init.sql"
	@echo "  make tidy    go mod tidy"
	@echo "  make build   Compila el binario en bin/api"
	@echo "  make run     Ejecuta el servidor (necesita DB levantada: make -C .. dev-db)"
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
