# =============================================================
# Stage 1: build del bundle estático
# =============================================================
FROM node:22-alpine AS builder

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
FROM nginx:1.27-alpine

# Reemplazamos la config por defecto.
COPY nginx.conf /etc/nginx/conf.d/default.conf

COPY --from=builder /src/dist /usr/share/nginx/html

EXPOSE 80

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=3 \
    CMD wget -q -O - http://localhost/ >/dev/null 2>&1 || exit 1
