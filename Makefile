.PHONY: run build migrate-up migrate-down sqlc-gen seed test test-integration

run:
	go run ./cmd/server

build:
	go build -o bin/server ./cmd/server

migrate-up:
	migrate -path db/migrations -database "$(DATABASE_URL)" up

migrate-down:
	migrate -path db/migrations -database "$(DATABASE_URL)" down 1

sqlc-gen:
	sqlc generate

seed:
	psql "$(DATABASE_URL)" -f db/seed.sql

test:
	go test ./...

test-integration:
	TEST_DATABASE_URL="$(TEST_DATABASE_URL)" go test -tags=integration ./internal/integration/ -v
