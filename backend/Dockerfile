# =============================================================
# Stage 1: build estático del binario Go
# =============================================================
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Capa cacheable: si solo cambian fuentes (no deps) se reusa.
COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0 -> binario estático (no depende de glibc/musl).
# -ldflags "-w -s" -> stripped, ~30% más chico.
# -trimpath -> rutas reproducibles, no leakean /home/...
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-w -s" \
    -o /out/api ./cmd/api

# =============================================================
# Stage 2: runtime slim (~25 MB total)
# =============================================================
FROM alpine:3.20

RUN apk add --no-cache \
        ca-certificates \
        tzdata \
        wget \
    && addgroup -S app \
    && adduser -S app -G app

WORKDIR /app
COPY --from=builder /out/api /app/api

USER app
EXPOSE 8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O - http://localhost:8080/api/health || exit 1

ENTRYPOINT ["/app/api"]
