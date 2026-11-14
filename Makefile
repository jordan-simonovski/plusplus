SHELL := /bin/sh

APP_ENTRY := ./cmd/server

.PHONY: fmt fmt-check lint test check test-integration run build up down logs release

# Go code lives in cmd/ and internal/ (the Dockerfile copies only those). Scoping to them
# keeps gofmt out of .claude/worktrees and node_modules.
GO_DIRS := cmd internal

fmt:
	gofmt -w $(GO_DIRS)

fmt-check:
	@out="$$(gofmt -l $(GO_DIRS))" || exit 1; if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi

lint:
	go vet ./...
	go vet -tags=integration ./internal/persistence
	golangci-lint run ./...

test:
	go test -race ./...

check: fmt-check lint test

test-integration:
	DYNAMODB_ENDPOINT=http://localhost:8000 AWS_REGION=ap-southeast-2 AWS_ACCESS_KEY_ID=local AWS_SECRET_ACCESS_KEY=local go test -count=1 -tags=integration ./internal/persistence -v

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

release:
	npm run release
