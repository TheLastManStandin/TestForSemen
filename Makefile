SQLC_IMAGE := sqlc/sqlc:1.31.1

.PHONY: up down logs seed test test-short lint gen gen-sql gen-api gen-web eval eval-all

up:
	docker compose up --build -d

down:
	docker compose down

# Seeding only happens when the expenses table is empty.
seed:
	SEED_DEMO=true docker compose up -d --build backend

logs:
	docker compose logs -f backend

test:
	cd backend && go test ./...

test-short:
	cd backend && go test -short ./...

lint:
	cd backend && docker run --rm -v "$$PWD":/app -v "$$(go env GOMODCACHE)":/go/pkg/mod -w /app golangci/golangci-lint:v2.14.0 golangci-lint run ./...
	cd frontend && npm run lint && npm run typecheck

gen: gen-sql gen-api gen-web

gen-sql:
	docker run --rm -u $$(id -u):$$(id -g) -v $(CURDIR)/backend:/src -w /src $(SQLC_IMAGE) generate

gen-api:
	cd backend && go tool oapi-codegen -config oapi-codegen.yaml ../api/openapi.yaml

gen-web:
	npm --prefix frontend run gen:api

EVAL_MODELS := anthropic/claude-haiku-5.5 openai/gpt-6-luna google/gemini-3.8-flash deepseek/deepseek-v4.1-flash qwen/qwen3.8-omni-flash

eval:
	cd backend && { [ ! -f ../.env ] || { set -a; . ../.env; set +a; }; } && go run ./cmd/eval $(if $(MODEL),-model "$(MODEL)")

eval-all:
	for m in $(EVAL_MODELS); do $(MAKE) eval MODEL=$$m || exit 1; done
