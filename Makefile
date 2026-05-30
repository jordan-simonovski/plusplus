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
	DYNAMODB_ENDPOINT=http://localhost:8000 AWS_REGION=ap-southeast-2 AWS_ACCESS_KEY_ID=local AWS_SECRET_ACCESS_KEY=local go test -tags=integration ./internal/persistence -v

run:
	PORT=8080 AWS_REGION=ap-southeast-2 DYNAMODB_ENDPOINT=http://localhost:8000 AWS_ACCESS_KEY_ID=local AWS_SECRET_ACCESS_KEY=local SLACK_SIGNING_SECRET=dev-signing-secret SLACK_BOT_TOKEN=xoxb-local-token go run $(APP_ENTRY)

build:
	go build ./...

up:
	docker compose up --build -d

down:
	docker compose down -v

logs:
	docker compose logs -f app
