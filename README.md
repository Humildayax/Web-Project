# Security Portal

[![CI](https://github.com/Humildayax/Web-Project/actions/workflows/ci.yml/badge.svg)](https://github.com/Humildayax/Web-Project/actions/workflows/ci.yml)

Portal interno con tres secciones:

- **Noticias**: feed RSS externo cacheado con TTL.
- **Reportar**: formulario que crea un incidente en Postgres y abre un ticket en JIRA (resiliente a JIRA caído).
- **Políticas**: visor de PDF estático servido desde `frontend/public/policies.pdf`.

Stack: Go (chi + pgx + sqlc) + React (Vite + TypeScript) + Postgres + Nginx, todo orquestado con Docker Compose.

## Cómo correr

```bash
cp .env.example .env        # editá DB_PASSWORD y, si querés JIRA real, JIRA_*
make up                     # levanta db + backend + frontend
open http://localhost
```

Apagar y limpiar:

```bash
make down                   # baja contenedores, conserva la DB
make clean                  # también borra el volumen de Postgres
```

## Modos de desarrollo

```bash
make dev-db                 # solo Postgres en Docker, expuesto en :5433
make dev-back               # ejecuta el backend nativo (go run ./cmd/api)
make dev-front              # ejecuta vite en :5173 con hot-reload
```

El backend lee `.env` desde el cwd y, si no lo encuentra, sube un nivel — así funciona desde la raíz y desde `backend/`.

## Estructura

```
.
├── backend/
│   ├── cmd/api/             # entrypoint, bootstrap, router
│   ├── internal/
│   │   ├── config/          # carga de env vars con defaults
│   │   ├── db/              # código generado por sqlc
│   │   ├── dto/             # request/response + validator
│   │   ├── handlers/        # HTTP handlers + middlewares (Logging, Recover)
│   │   ├── jira/            # cliente JIRA + noop para dev
│   │   ├── migrate/         # runner de migraciones (golang-migrate)
│   │   ├── models/          # tipos de dominio
│   │   ├── news/            # provider RSS
│   │   ├── repository/      # acceso a Postgres con pgx
│   │   └── services/        # lógica de negocio + workers
│   ├── migrations/          # *.up.sql / *.down.sql, embebidas en el binario
│   ├── db/queries/          # SQL fuente para sqlc
│   ├── sqlc.yaml
│   └── Dockerfile
├── frontend/
│   ├── src/
│   │   ├── api/             # cliente HTTP único (fetch wrapper)
│   │   ├── components/
│   │   └── pages/
│   ├── nginx.conf           # proxy /api -> backend, headers de seguridad
│   └── Dockerfile
├── docker-compose.yml
├── Makefile
└── .env.example
```

## Decisiones de diseño que conviene conocer

- **DB-first ante JIRA caído**: `POST /api/incidents` siempre persiste primero en Postgres. Si la llamada a JIRA falla, el cliente recibe 201 y un worker (`internal/services/jira_retry_worker.go`) reintenta de fondo con backoff exponencial. Las consultas usan `FOR UPDATE SKIP LOCKED` así que es seguro correr varias réplicas del backend.
- **Migraciones en runtime**: el binario corre `golang-migrate` al arrancar (var `MIGRATE_ON_START`, default `true`). Postgres en compose ya no monta `init.sql`. Para crear una nueva: `cd backend && make migrate-create name=algo`.
- **Retención de PII**: `internal/services/retention_worker.go` pone `metadata = NULL` en incidentes con `created_at < NOW() - INCIDENT_METADATA_MAX_AGE` (default 90d). La fila se conserva, solo desaparece la IP/UA del reporte.
- **Rate-limiting en dos capas**: nginx con `limit_req` (10r/s burst 20 sobre `/api/`) y el backend con `httprate` por IP solo en `POST /api/incidents` (10/min default). El IP se obtiene de `X-Real-IP` (que setea el propio nginx), no del primer valor de `X-Forwarded-For`.
- **CORS opt-out**: `ALLOWED_ORIGINS=` (vacío explícito) deshabilita el middleware. En prod el front y back van por el mismo origen vía nginx, no hace falta CORS.
- **Pinneo Docker por digest**: las cinco imágenes base están fijadas con `@sha256:...` para builds reproducibles.
- **`X-Request-Id`**: middleware de chi genera un ID por request, se loguea y se devuelve en el header de respuesta.
- **`X-Cache: stale`**: si las noticias se sirven desde cache porque el provider RSS falló, la respuesta lleva ese header.

## Endpoints

| Método | Ruta              | Descripción                                                    |
|--------|-------------------|----------------------------------------------------------------|
| GET    | `/api/health`     | Liveness. 200 si el proceso responde, no toca DB.              |
| GET    | `/api/ready`      | Readiness. 200 si la DB responde, 503 si no.                   |
| GET    | `/api/news`       | Lista de noticias cacheada (TTL `NEWS_CACHE_TTL`).             |
| POST   | `/api/incidents`  | Crea un incidente. Rate-limited por IP.                        |

## Variables de entorno

Ver `.env.example`. Las que no están comentadas son requeridas; las comentadas tienen default razonable.

## Migraciones manualmente

El binario las corre solo, pero para rollback o debugging:

```bash
cd backend
go install -tags 'pgx5' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
make migrate-status
make migrate-up
make migrate-down       # reversa la última
```

## Regenerar código sqlc

Si tocás `db/queries/*.sql` o `migrations/*.up.sql`:

```bash
go install github.com/sqlc-dev/sqlc/cmd/sqlc@latest
cd backend
make sqlc
```

## CI

`.github/workflows/ci.yml` corre en cada PR y push a `main`. Tres jobs:

| Job | Pasos |
|-----|-------|
| `backend`  | `go vet`, `go build`, `go test -race`, `golangci-lint` |
| `frontend` | `npm ci`, `npx tsc -b`, `npm run build` |
| `docker`   | `docker build` para backend y frontend (verifica que los Dockerfiles compilen con los digests pinneados) |

Para reproducir localmente:

```bash
# backend
cd backend && go vet ./... && go build ./... && go test -race ./...
go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest
golangci-lint run

# frontend
cd frontend && npm ci && npx tsc -b && npm run build
```

## Refrescar digests de imágenes Docker

Cuando suban tags nuevos de las base images, los SHAs cambian. Para obtener los nuevos:

```bash
for img in alpine:3.20 golang:1.26-alpine node:22-alpine nginx:1.27-alpine postgres:15-alpine; do
  echo -n "$img -> "
  docker manifest inspect $img -v | jq -r '.Descriptor.digest // .[0].Descriptor.digest' | head -1
done
```

Reemplazá los SHAs en `backend/Dockerfile`, `frontend/Dockerfile` y `docker-compose.yml`.
