# =============================================================================
# GAH Server — Dockerfile (multi-stage)
# =============================================================================
# O fluxo PRINCIPAL de desenvolvimento é `go run`/`make run` rodando no host
# contra um Postgres local. Este Dockerfile existe para empacotar o serviço de
# forma reproduzível (CI, deploy, ou rodar tudo via docker compose).
#
# O driver de banco (jackc/pgx stdlib) é Go puro, então buildamos com
# CGO_ENABLED=0 e geramos um binário estático que roda numa imagem mínima.
#
# Build (contexto = ./server):
#   docker build -t gah-server ./server
# Run (DATABASE_URL sobrescreve a config — ver internal/infra/config):
#   docker run --rm -p 8080:8080 -e DATABASE_URL=... gah-server
# =============================================================================

# ---- Stage 1: build -----------------------------------------------------------
FROM golang:1.25-alpine AS build

WORKDIR /src

# Cache de dependências: copia só os manifests primeiro.
COPY go.mod go.sum ./
RUN go mod download

# Copia o restante do código e compila um binário estático.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
        -ldflags="-s -w" \
        -o /out/gah-server ./cmd/api

# ---- Stage 2: runtime ---------------------------------------------------------
FROM alpine:3.20 AS runtime

# Certificados (TLS p/ Postgres gerenciado) e timezone.
RUN apk add --no-cache ca-certificates tzdata wget \
    && adduser -D -u 10001 gah

WORKDIR /app

# Binário e migrations (a app não roda migrations; ficam disponíveis p/ ops).
COPY --from=build /out/gah-server /app/gah-server
COPY migrations /app/migrations

USER gah

EXPOSE 8080

# Sem config.toml a app usa defaults + overrides via env (DATABASE_URL, PORT...).
ENV PORT=8080

HEALTHCHECK --interval=10s --timeout=3s --start-period=5s --retries=5 \
    CMD wget -qO- http://localhost:8080/health || exit 1

ENTRYPOINT ["/app/gah-server"]
