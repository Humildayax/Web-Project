.PHONY: help up down restart logs ps build dev-db dev-back dev-front clean

help:
	@echo "Stack completo (prod-like, todo en Docker):"
	@echo "  make up           Levanta db + backend + frontend"
	@echo "  make down         Apaga todo"
	@echo "  make restart      down + up"
	@echo "  make logs         Logs en vivo"
	@echo "  make ps           Estado de los servicios"
	@echo "  make build        Rebuild de las imágenes"
	@echo ""
	@echo "Desarrollo (solo db en Docker, back y front nativos):"
	@echo "  make dev-db       Solo Postgres"
	@echo "  make dev-back     go run del backend"
	@echo "  make dev-front    npm run dev del frontend"
	@echo ""
	@echo "  make clean        down -v (BORRA volúmenes incluida la DB)"

up:
	docker compose up -d --build

down:
	docker compose down

restart: down up

logs:
	docker compose logs -f --tail=100

ps:
	docker compose ps

build:
	docker compose build

dev-db:
	docker compose -f docker-compose.yml -f docker-compose.dev.yml up -d db

dev-back:
	$(MAKE) -C backend run

dev-front:
	cd frontend && npm run dev

clean:
	docker compose down -v
