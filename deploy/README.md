# Deploy / execução local

O GAH server roda de **duas formas**. O caminho principal de desenvolvimento é
`go run` no host; o Docker existe para empacotamento reproduzível e para subir a
stack completa.

## 1. Desenvolvimento com `go run` (recomendado)

Sobe apenas o Postgres em container e roda a aplicação no host:

```bash
make db-up            # docker compose up -d db  (Postgres na :5432)
cd server && make run # go build + go run no host
```

A app lê `server/config.toml` (ou defaults) e aceita overrides por env. Para
apontar no Postgres do compose:

```bash
DATABASE_URL=postgres://gah:gah@localhost:5432/gah?sslmode=disable make run
```

## 2. Stack completa em container

```bash
make docker-up        # build da imagem + Postgres + servidor
# ou: docker compose -f deploy/docker-compose.yml up --build
```

Servidor em `http://localhost:8080` (healthcheck em `/health`).

## Migrations e seed

As migrations (`server/migrations/*.sql`) são aplicadas automaticamente na
**primeira** inicialização do volume do Postgres. Para recriar do zero:

```bash
make docker-down ARGS=-v   # apaga o volume; o próximo up reaplica as migrations
```

O seed de testes manuais é aplicado à parte — ver
[`manual_tests/sprint_1/`](../manual_tests/sprint_1/).

## Variáveis de ambiente reconhecidas

`PORT`, `DATABASE_URL`, `SERVER_READ_TIMEOUT`, `SERVER_WRITE_TIMEOUT`,
`SERVER_IDLE_TIMEOUT`, `DB_MAX_OPEN_CONNS`, `DB_MAX_IDLE_CONNS`,
`DB_CONN_MAX_LIFETIME` (ver `server/internal/infra/config/config.go`).
