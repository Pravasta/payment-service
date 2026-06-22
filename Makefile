.PHONY: help setup tidy build run dev worker migrate test fmt vet db-up db-down up down

help:
	@echo "Targets:"
	@echo "  setup    - salin .env.example -> .env"
	@echo "  tidy     - go mod tidy"
	@echo "  build    - build api, worker, migrate ke ./bin"
	@echo "  run      - jalankan API server (go run)"
	@echo "  dev      - jalankan API dengan Air (live reload)"
	@echo "  worker   - jalankan background worker"
	@echo "  migrate  - jalankan GORM AutoMigrate"
	@echo "  test     - go test ./..."
	@echo "  fmt/vet  - format & vet"
	@echo "  db-up    - start hanya Postgres (docker compose)"
	@echo "  up/down  - start/stop semua service (docker compose)"

setup:
	@test -f .env || (cp .env.example .env && echo ".env dibuat dari .env.example")

tidy:
	go mod tidy

dev:
	@command -v air >/dev/null 2>&1 || (echo "air belum terinstall: go install github.com/air-verse/air@latest" && exit 1)
	air

build:
	go build -o bin/api     ./cmd/api
	go build -o bin/worker  ./cmd/worker
	go build -o bin/migrate ./cmd/migrate

run:
	go run ./cmd/api

worker:
	go run ./cmd/worker

migrate:
	go run ./cmd/migrate

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

db-up:
	docker compose up -d postgres

db-down:
	docker compose stop postgres

up:
	docker compose up -d --build

down:
	docker compose down
