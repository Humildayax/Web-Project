# =============================================================
# Stage 1: build del bundle estático
# =============================================================
# Pin por digest para builds reproducibles. Refrescar manualmente cuando
# se actualice el tag node:22-alpine.
FROM node:22-alpine@sha256:8ea2348b068a9544dae7317b4f3aafcdc032df1647bb7d768a05a5cad1a7683f AS builder

WORKDIR /src

# Capa cacheable de deps.
COPY package.json package-lock.json* ./
RUN npm ci

COPY . .

# vite build lee .env.production (VITE_API_BASE_URL="" -> URLs relativas).
RUN npm run build

# =============================================================
# Stage 2: nginx sirve el bundle + proxy /api -> backend
# =============================================================
FROM nginx:1.27-alpine@sha256:65645c7bb6a0661892a8b03b89d0743208a18dd2f3f17a54ef4b76fb8e2f2a10

# Reemplazamos la config por defecto.
COPY nginx.conf /etc/nginx/conf.d/default.conf

COPY --from=builder /src/dist /usr/share/nginx/html

EXPOSE 80

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O - http://localhost/ >/dev/null 2>&1 || exit 1
