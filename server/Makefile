.PHONY: build run test test-cover lint fmt vet check ci clean \
        docker-build docker-up docker-down db-up db-down

BINARY_NAME=gah-server
BUILD_DIR=./bin
COMPOSE=docker compose -f ../deploy/docker-compose.yml

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
