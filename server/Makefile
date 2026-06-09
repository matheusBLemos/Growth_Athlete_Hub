.PHONY: build run test test-cover test-integration lint fmt vet check ci clean \
        docker-build docker-up docker-down db-up db-down stack-up \
        db-wait seed dev smoke migrate

BINARY_NAME=gah-server
BUILD_DIR=./bin
COMPOSE=docker compose -f ../deploy/docker-compose.yml
# DSN usado pelo `go run` no host para alcançar o Postgres do compose.
DATABASE_URL?=postgres://gah:gah@localhost:5432/gah?sslmode=disable

# Endpoints dos serviços do compose usados pelos testes de integração. Apontam
# para os mesmos serviços de dev (db/rabbitmq/redis), nas portas expostas no host.
TEST_DATABASE_URL?=postgres://gah:gah@localhost:5432/gah?sslmode=disable
TEST_REDIS_ADDR?=localhost:6379
TEST_RABBITMQ_URL?=amqp://gah:gah@localhost:5672/

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

# Testes de integração (//go:build integration) contra infra REAL. Cada suíte
# pula sozinha se a sua env (TEST_DATABASE_URL/TEST_REDIS_ADDR/TEST_RABBITMQ_URL)
# não estiver setada. As envs apontam, por padrão, para os serviços do compose
# (`make stack-up` sobe db+rabbitmq+redis). Rode `make stack-up` antes.
test-integration:
	@echo ">> rodando testes de integração contra a infra do compose"
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" \
	TEST_REDIS_ADDR="$(TEST_REDIS_ADDR)" \
	TEST_RABBITMQ_URL="$(TEST_RABBITMQ_URL)" \
	go test -tags=integration -count=1 ./...

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

# Sobe a stack completa de infra (Postgres + RabbitMQ + Redis) em background,
# sem o servidor/worker — usado por `make dev` e `make test-integration`.
stack-up:
	$(COMPOSE) up -d db rabbitmq redis

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

# Aplica o seed via o seeder Go (cmd/seed): cria os DOIS usuários prontos para
# login (senha real com o mesmo Argon2id + pepper da API), idempotente. Substitui
# o antigo seed.sql, que inseria usuários sem senha. O seeder roda Migrate por
# conta própria, mas mantemos a dependência num banco vivo (`db-up`/`db-wait`).
seed: db-up db-wait
	@echo ">> aplicando seed (cmd/seed) em $(DATABASE_URL)"
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/seed
	@echo ">> seed aplicado."

# Fluxo único de dev: sobe a stack completa (db+rabbitmq+redis), espera o banco,
# aplica o seed e roda a API (go run no host).
dev: stack-up db-wait seed
	@echo ">> iniciando a API (go run) em :8080"
	DATABASE_URL="$(DATABASE_URL)" go run ./cmd/api

# Smoke test ponta a ponta da Sprint 2 (auth-first): assume a API já no ar em
# :8080 (ex.: `make dev`) e os usuários semeados por `make seed`.
smoke:
	../manual_tests/sprint_2/smoke_test.sh
