SQLC_IMAGE := sqlc/sqlc:1.31.1

.PHONY: up down logs seed test test-short lint gen gen-sql gen-api gen-web eval

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

eval:
	cd backend && go run ./cmd/eval -model "$(MODEL)"
