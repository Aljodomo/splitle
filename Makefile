.PHONY: all build test test-e2e test-postgres up down clean

DB_URL ?= postgres://splitle:splitle_secret@localhost:5432/splitle_db?sslmode=disable

all: test

up:
	docker compose up -d

down:
	docker compose down

test:
	go test -v -race ./internal/... ./test/...

test-postgres: up
	@echo "Waiting for postgres to be ready..."
	@docker compose exec postgres pg_isready -U splitle -d splitle_db
	TEST_POSTGRES_URL="$(DB_URL)" go test -v -race ./internal/adapters/storage/postgres/...

clean:
	go clean
