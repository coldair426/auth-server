.PHONY: run build migrate-up migrate-down sqlc-gen seed

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
