.PHONY: build run test test-cover lint fmt vet check ci clean \
        docker-build docker-up docker-down db-up db-down \
        db-wait seed dev smoke migrate

BINARY_NAME=gah-server
BUILD_DIR=./bin
COMPOSE=docker compose -f ../deploy/docker-compose.yml
SEED_FILE=../manual_tests/sprint_1/seed.sql
# DSN usado pelo `go run` no host para alcançar o Postgres do compose.
DATABASE_URL?=postgres://gah:gah@localhost:5432/gah?sslmode=disable

build:
	@mkdir -p $(BUILD_DIR)
	go build -o $(BUILD_DIR)/$(BINARY_NAME) ./cmd/api

run: build
	$(BUILD_DIR)/$(BINARY_NAME)

test:
	go test ./... -v -race

test-cover:
	go test ./... -v -race -coverprofile=coverage.out
	go tool cover -func=coverage.out

lint:
	golangci-lint run ./...

fmt:
	gofmt -l -d .

fmt-fix:
	gofmt -w .

vet:
	go vet ./...

check: fmt vet lint test

ci: check

clean:
	rm -rf $(BUILD_DIR) coverage.out

# --- Docker / banco de dados -------------------------------------------------
# Fluxo recomendado de dev: `make db-up` e depois `make run` (go run no host).
docker-build:
	docker build -t $(BINARY_NAME) .

# Sobe a stack completa (Postgres + servidor) em container.
docker-up:
	$(COMPOSE) up --build

# Derruba a stack (use `make docker-down ARGS=-v` p/ apagar o volume do banco).
docker-down:
	$(COMPOSE) down $(ARGS)

# Sobe SÓ o Postgres (para desenvolver a app com `make run`/go run no host).
db-up:
	$(COMPOSE) up -d db

db-down:
	$(COMPOSE) stop db

# Aguarda o Postgres aceitar conexões (healthcheck do compose já roda pg_isready).
db-wait:
	@echo ">> aguardando o Postgres ficar pronto..."
	@until $(COMPOSE) exec -T db pg_isready -U gah -d gah >/dev/null 2>&1; do \
		printf '.'; sleep 1; \
	done; echo " ok"

# Aplica as migrations embutidas (schema_migrations) no banco de dev, via runner
# Go no host. Idempotente. Requer o Postgres/TimescaleDB no ar (`make db-up`).
migrate:
	@echo ">> aplicando migrations em $(DATABASE_URL)"
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/migrate

# Aplica o seed de testes manuais dentro do container (host não tem psql).
# As migrations agora são aplicadas pelo runner Go (não mais pelo initdb), então
# o seed depende de `migrate` para garantir o schema antes de inserir dados.
seed: db-up db-wait migrate
	@echo ">> aplicando seed: $(SEED_FILE)"
	$(COMPOSE) exec -T db psql -v ON_ERROR_STOP=1 -U gah -d gah < $(SEED_FILE)
	@echo ">> seed aplicado."

# Fluxo único de dev: sobe o Postgres, espera, aplica o seed e roda a app (go run).
dev: seed
	@echo ">> iniciando a API (go run) em :8080"
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/api

# Smoke test ponta a ponta: assume a API já no ar em :8080 (ex.: `make dev`).
smoke:
	../manual_tests/sprint_1/smoke_test.sh
