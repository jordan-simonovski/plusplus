SHELL := /bin/sh

APP_ENTRY := ./cmd/server

.PHONY: fmt lint test test-integration run build up down logs

fmt:
	gofmt -w $$(find . -name '*.go' -not -path './vendor/*')

lint:
	go vet ./...

test:
	go test ./...

test-integration:
	DATABASE_URL=postgres://postgres:postgres@localhost:5432/plusplus?sslmode=disable go test -tags=integration ./internal/persistence -v

run:
	PORT=8080 DATABASE_URL=postgres://postgres:postgres@localhost:5432/plusplus?sslmode=disable SLACK_SIGNING_SECRET=dev-signing-secret SLACK_BOT_TOKEN=xoxb-local-token go run $(APP_ENTRY)

build:
	go build ./...

up:
	docker compose up --build -d

down:
	docker compose down -v

logs:
	docker compose logs -f app
