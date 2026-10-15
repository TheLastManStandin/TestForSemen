# CLAUDE.md

Family expenses service: Telegram bot with an AI agent + web panel for the head of the family.

- Spec: `docs/superpowers/specs/2026-10-15-family-expenses-design.md`
- Plans: `docs/superpowers/plans/`

## Layout
- `backend/` — Go module `github.com/TheLastManStandin/TestForSemen/backend`
- `frontend/` — React + Vite + TypeScript panel
- `api/openapi.yaml` — REST contract, the single source of truth for backend and frontend types

## Commands
- `make test` (all, needs Docker) / `make test-short` (unit only)
- `make gen` — regenerate sqlc, oapi-codegen, frontend API types
- `make up` / `make down` — docker compose
- `make eval [MODEL=<openrouter model id>]` — agent evals (model defaults to `$LLM_MODEL`); `make eval-all` — compare all candidate models and refresh `backend/evals/results/README.md`

## Rules
- Business logic lives only in `backend/internal/expense` (`Service`). HTTP and the agent are thin adapters.
- Never add `user_id` to agent tool schemas; the member comes from the Telegram update.
- Money is `domain.Money` (kopecks), never float. Dates are `domain.DateOf` values; "today" is `Service.Today()`.
- REST change: edit `api/openapi.yaml` → `make gen-api gen-web` → implement.
- SQL change: edit `backend/migrations` / `backend/internal/store/postgres/queries` → `make gen-sql`.
- Generated code (`internal/store/postgres/db`, `internal/httpapi/gen`, `frontend/src/api/schema.d.ts`) is committed and never edited by hand.
- TDD: write the failing test first. Integration tests use testcontainers and skip under `-short`.
- User-facing strings are Russian; code and comments are English.
- Conventional Commits.
