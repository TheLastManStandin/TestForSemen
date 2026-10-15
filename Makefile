SQLC_IMAGE := sqlc/sqlc:1.31.1

.PHONY: up down logs test test-short lint gen gen-sql gen-api gen-web eval

up:
	docker compose up --build -d

down:
	docker compose down

logs:
	docker compose logs -f backend

test:
	cd backend && go test ./...

test-short:
	cd backend && go test -short ./...

lint:
	cd backend && go vet ./...
	cd frontend && npm run lint && npm run typecheck

gen: gen-sql gen-api gen-web

gen-sql:
	docker run --rm -u $$(id -u):$$(id -g) -v $(CURDIR)/backend:/src -w /src $(SQLC_IMAGE) generate

gen-api:
	cd backend && go tool oapi-codegen -config oapi-codegen.yaml ../api/openapi.yaml

gen-web:
	cd frontend && npx openapi-typescript ../api/openapi.yaml -o src/api/schema.d.ts

eval:
	cd backend && go run ./cmd/eval -model "$(MODEL)"
