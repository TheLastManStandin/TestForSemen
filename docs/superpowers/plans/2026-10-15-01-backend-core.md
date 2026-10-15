# Backend Core Implementation Plan (Plan 1 of 4)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** A tested Go backend core — domain model, Postgres persistence, expense service with per-member scoping, and the REST API + SSE stream the web panel uses.

**Architecture:** One Go module in `backend/`. `internal/expense.Service` holds all business rules and depends on a `Repository` interface with two implementations (Postgres via sqlc, in-memory for tests/evals) verified by one shared contract test suite. `internal/httpapi` is a thin adapter generated from `api/openapi.yaml` by oapi-codegen (strict std-http server). Mutations publish events to an in-process bus that feeds SSE.

**Tech Stack:** Go 1.26, pgx v5, sqlc 1.31 (Docker), goose v3, oapi-codegen v2 (strict, net/http), testify, testcontainers-go.

**Spec:** `docs/superpowers/specs/2026-10-15-family-expenses-design.md` (sections 2, 3, 5, 8, 10, 11, 12). Overview: `docs/superpowers/plans/2026-10-15-00-overview.md`.

## Global Constraints

- Go module path: `github.com/TheLastManStandin/TestForSemen/backend` (module root `backend/`); `go 1.26`.
- Pinned deps: pgx `v5.11.0`, goose `v3.29.0`, oapi-codegen `v2.8.0` (installed as `go tool`), oapi-codegen runtime `v1.7.0`, testcontainers-go `v0.44.0`, caarlos0/env `v11.4.1`, testify `v1.12.1`.
- sqlc `1.31.1` via Docker image `sqlc/sqlc:1.31.1`; generated code committed.
- Postgres image `postgres:16` (Debian; needed for Cyrillic case-insensitive `ILIKE`).
- Money = `domain.Money` (int64 kopecks). API amount = decimal string `"350.00"`. Human format `"1 200 ₽"` / `"1 200,50 ₽"`.
- Category codes: `food, transport, housing, entertainment, health, shopping, other` ↔ еда, транспорт, жильё, развлечения, здоровье, покупки, другое.
- Domain dates are `time.Time` at 00:00 UTC (`domain.DateOf`); "today" is computed in `APP_TIMEZONE` (default `Europe/Moscow`).
- Own-scope access to another member's expense returns `domain.ErrNotFound`.
- Validation messages are Russian; identifiers/comments English.
- Conventional Commits, no attribution trailers.
- Integration tests (testcontainers) skip under `go test -short`.

## Review Focus

1. **Cyrillic case-insensitive search** — `text: "такси"` must find `"Такси домой"` in Postgres too (locale-dependent `ILIKE`). Pinned by the contract test "list filters and order" running against Postgres (Task 5).
2. **LIKE wildcards in user text** — searching `"%"` must not match everything. Pinned by contract test "count ignores pagination" (`Text: "%"` → 0) and `escapeLike` in Task 5.
3. **Empty lists serialize as `[]`, not `null`** — the frontend iterates `items`. Pinned by `TestListExpensesEmptyIsArray` (Task 8).
4. **Offset past the end** keeps correct `total`/`sum`. Pinned by `TestListExpensesOffsetBeyondTotal` (Task 8).
5. **SSE through the logging middleware** — a `ResponseWriter` wrapper silently breaks flushing. Pinned by `TestEventsStream` hitting the full handler chain (Task 9). Midnight-in-Moscow "today" is pinned by `TestTodayUsesLocation` (Task 7).

## File Map

```
.gitignore, .env.example, Makefile, CLAUDE.md            Task 1
api/openapi.yaml                                          Task 8
backend/
  go.mod, go.sum                                          Task 1 (+deps in later tasks)
  sqlc.yaml, oapi-codegen.yaml                            Task 5, Task 8
  migrations/00001_init.sql, migrations/embed.go          Task 5
  internal/domain/money.go (+_test)                       Task 1
  internal/domain/category.go, date.go, period.go,
                  errors.go, types.go (+_test)            Task 2
  internal/config/config.go (+_test)                      Task 3
  internal/expense/repository.go                          Task 4
  internal/expense/expensetest/contract.go                Task 4
  internal/expense/memrepo/memrepo.go (+_test)            Task 4
  internal/store/postgres/queries/*.sql, db/ (generated)  Task 5
  internal/store/postgres/postgres.go, expenses.go,
                  members.go, agent.go (+_test)           Task 5
  internal/testutil/pgtest/pgtest.go                      Task 5
  internal/events/bus.go (+_test)                         Task 6
  internal/expense/service.go, stats.go (+_test)          Task 7
  internal/httpapi/gen/api.gen.go (generated)             Task 8
  internal/httpapi/server.go, convert.go, errors.go,
                  middleware.go, handler.go (+_test)      Task 8
  internal/httpapi/sse.go, health.go; handler.go update   Task 9
```

---

### Task 1: Repository scaffold and `domain.Money`

**Files:**
- Create: `.gitignore`, `.env.example`, `Makefile`, `CLAUDE.md`
- Create: `backend/go.mod` (via `go mod init`)
- Create: `backend/internal/domain/money.go`
- Test: `backend/internal/domain/money_test.go`

**Interfaces:**
- Consumes: nothing.
- Produces: `type domain.Money int64`; `func domain.ParseMoney(s string) (domain.Money, error)`; `func (domain.Money) String() string` → `"1200.50"`; `func (domain.Money) Human() string` → `"1 200,50 ₽"`.

- [ ] **Step 1: Create root files**

**File:** `.gitignore`
```gitignore
.env
*.out
coverage.*
backend/bin/
frontend/node_modules/
frontend/dist/
.DS_Store
.idea/
.vscode/
```

**File:** `.env.example`
```dotenv
# Telegram bot token from @BotFather
TELEGRAM_TOKEN=
# Family members allowed to use the bot: TelegramID:Name, comma-separated
FAMILY_MEMBERS=123456789:Папа,987654321:Мама

# LLM (any OpenAI-compatible API; OpenRouter by default)
LLM_API_KEY=
LLM_BASE_URL=https://openrouter.ai/api/v1
LLM_MODEL=anthropic/claude-haiku-5.5
LLM_TIMEOUT=30s

# Voice messages (OpenRouter /audio/transcriptions, same key)
STT_ENABLED=true
STT_MODEL=openai/whisper-large-v3
STT_MAX_SECONDS=60

APP_TIMEZONE=Europe/Moscow
HTTP_ADDR=:8080
LOG_LEVEL=info
# Fill the panel with 60 days of demo expenses on first start (only if empty)
SEED_DEMO=false

# Used only when running the backend outside Docker; docker compose overrides it
DATABASE_URL=postgres://expenses:expenses@localhost:5432/expenses?sslmode=disable
```

**File:** `Makefile` (recipe lines start with a TAB)
```make
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
```

**File:** `CLAUDE.md`
```markdown
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
- `make eval MODEL=<openrouter model id>` — agent evals

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
```

- [ ] **Step 2: Initialise the Go module**

```bash
mkdir -p backend && cd backend
go mod init github.com/TheLastManStandin/TestForSemen/backend
go mod edit -go=1.26
go get github.com/stretchr/testify@v1.12.1
```

- [ ] **Step 3: Write the failing test**

**File:** `backend/internal/domain/money_test.go`
```go
package domain_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

func TestParseMoney(t *testing.T) {
	cases := []struct {
		in   string
		want domain.Money
	}{
		{"350", 35000},
		{"350.5", 35050},
		{"350,50", 35050},
		{"350.00", 35000},
		{"1 200", 120000},
		{"1 200,50 ₽", 120050},
		{"1\u00a0200", 120000},
		{"0.99", 99},
		{".5", 50},
	}
	for _, c := range cases {
		t.Run(c.in, func(t *testing.T) {
			got, err := domain.ParseMoney(c.in)
			require.NoError(t, err)
			assert.Equal(t, c.want, got)
		})
	}
}

func TestParseMoneyRejects(t *testing.T) {
	for _, in := range []string{"", " ", "₽", "abc", "-5", "1.234", "12.3.4", "1e3", "1234567890123456"} {
		_, err := domain.ParseMoney(in)
		assert.Error(t, err, "input %q", in)
	}
}

func TestMoneyString(t *testing.T) {
	assert.Equal(t, "350.00", domain.Money(35000).String())
	assert.Equal(t, "350.50", domain.Money(35050).String())
	assert.Equal(t, "0.99", domain.Money(99).String())
	assert.Equal(t, "0.00", domain.Money(0).String())
}

func TestMoneyHuman(t *testing.T) {
	assert.Equal(t, "350 ₽", domain.Money(35000).Human())
	assert.Equal(t, "1 200 ₽", domain.Money(120000).Human())
	assert.Equal(t, "1 200,50 ₽", domain.Money(120050).Human())
	assert.Equal(t, "1 234 567,89 ₽", domain.Money(123456789).Human())
	assert.Equal(t, "0,99 ₽", domain.Money(99).Human())
}
```

- [ ] **Step 4: Run test to verify it fails**

Run: `cd backend && go test ./internal/domain/`
Expected: FAIL — `undefined: domain.ParseMoney` (package does not compile).

- [ ] **Step 5: Implement**

**File:** `backend/internal/domain/money.go`
```go
package domain

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// Money is an amount in kopecks (1/100 of a rouble).
type Money int64

var errBadAmount = errors.New("некорректная сумма")

var separatorStripper = strings.NewReplacer(" ", "", "\u00a0", "", "\u202f", "", "_", "")

// ParseMoney parses amounts like "350", "350.5", "350,50", "1 200", "1 200,50 ₽".
func ParseMoney(s string) (Money, error) {
	clean := strings.TrimSpace(s)
	clean = strings.TrimSpace(strings.TrimSuffix(clean, "₽"))
	clean = separatorStripper.Replace(clean)
	clean = strings.Replace(clean, ",", ".", 1)
	if clean == "" {
		return 0, fmt.Errorf("%w: пустое значение", errBadAmount)
	}

	whole, frac, hasFrac := strings.Cut(clean, ".")
	if whole == "" {
		whole = "0"
	}
	if !isDigits(whole) || len(whole) > 15 {
		return 0, fmt.Errorf("%w: %q", errBadAmount, s)
	}
	switch {
	case !hasFrac:
		frac = "00"
	case len(frac) == 1 && isDigits(frac):
		frac += "0"
	case len(frac) == 2 && isDigits(frac):
	default:
		return 0, fmt.Errorf("%w: %q", errBadAmount, s)
	}

	w, err := strconv.ParseInt(whole, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%w: %q", errBadAmount, s)
	}
	f, _ := strconv.ParseInt(frac, 10, 64)
	return Money(w*100 + f), nil
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// String renders the API representation, e.g. "1200.50".
func (m Money) String() string {
	return fmt.Sprintf("%d.%02d", int64(m)/100, int64(m)%100)
}

// Human renders the user-facing representation, e.g. "1 200 ₽" or "1 200,50 ₽".
func (m Money) Human() string {
	whole := groupThousands(int64(m) / 100)
	if kop := int64(m) % 100; kop != 0 {
		return fmt.Sprintf("%s,%02d ₽", whole, kop)
	}
	return whole + " ₽"
}

func groupThousands(n int64) string {
	s := strconv.FormatInt(n, 10)
	var b strings.Builder
	for i, r := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte(' ')
		}
		b.WriteRune(r)
	}
	return b.String()
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd backend && go test ./internal/domain/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add .gitignore .env.example Makefile CLAUDE.md backend/go.mod backend/go.sum backend/internal/domain
git commit -m "feat(domain): add money type and project scaffold"
```

---

### Task 2: Domain categories, dates, periods, errors, types

**Files:**
- Create: `backend/internal/domain/errors.go`, `category.go`, `date.go`, `period.go`, `types.go`
- Test: `backend/internal/domain/category_test.go`, `period_test.go`

**Interfaces:**
- Consumes: nothing new.
- Produces:
  - `var domain.ErrNotFound error`; `type domain.ValidationError struct{ Field, Message string }`; `func domain.Invalid(field, message string) error`
  - `type domain.Category string`; constants `CategoryFood, CategoryTransport, CategoryHousing, CategoryEntertainment, CategoryHealth, CategoryShopping, CategoryOther`; `var domain.Categories []Category`; `func (Category) Valid() bool`; `func (Category) RU() string`; `func domain.ParseCategory(s string) (Category, error)`
  - `const domain.DateLayout = "2006-01-02"`; `func domain.DateOf(t time.Time) time.Time`; `func domain.ParseDate(s string) (time.Time, error)`; `func domain.FormatDate(t time.Time) string`; `func domain.WeekdayRU(t time.Time) string`
  - `type domain.Period string`; constants `PeriodToday, PeriodYesterday, PeriodThisWeek, PeriodLastWeek, PeriodThisMonth, PeriodLastMonth, PeriodLast7Days, PeriodLast30Days`; `var domain.Periods []Period`; `func domain.ResolvePeriod(p Period, now time.Time) (from, to time.Time, err error)`
  - `type domain.Expense struct{ ID, UserID int64; MemberName string; Amount Money; Category Category; SpentAt time.Time; Comment string; CreatedAt, UpdatedAt time.Time }`
  - `type domain.Member struct{ TelegramID int64; Name string }`
  - `type domain.Trace struct{ ID, UserID int64; MemberName, Source, Input, Output string; Steps []TraceStep; Model string; PromptTokens, CompletionTokens int; STTms *int; LatencyMs int; Error string; CreatedAt time.Time }`
  - `type domain.TraceStep struct{ Tool string; Args, Result json.RawMessage; Ms int64 }` (JSON tags `tool,args,result,ms`)

- [ ] **Step 1: Write the failing tests**

**File:** `backend/internal/domain/category_test.go`
```go
package domain_test

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

func TestParseCategory(t *testing.T) {
	cases := []struct {
		in   string
		want domain.Category
	}{
		{"food", domain.CategoryFood},
		{" Transport", domain.CategoryTransport},
		{"еда", domain.CategoryFood},
		{"Жильё", domain.CategoryHousing},
		{"жилье", domain.CategoryHousing},
		{"другое", domain.CategoryOther},
	}
	for _, c := range cases {
		got, err := domain.ParseCategory(c.in)
		require.NoError(t, err, c.in)
		assert.Equal(t, c.want, got, c.in)
	}
}

func TestParseCategoryRejectsUnknown(t *testing.T) {
	_, err := domain.ParseCategory("pets")
	var ve *domain.ValidationError
	require.True(t, errors.As(err, &ve))
	assert.Equal(t, "category", ve.Field)
}

func TestCategoryRU(t *testing.T) {
	assert.Equal(t, "развлечения", domain.CategoryEntertainment.RU())
	assert.Len(t, domain.Categories, 7)
	for _, c := range domain.Categories {
		assert.True(t, c.Valid(), c)
	}
	assert.False(t, domain.Category("pets").Valid())
}
```

**File:** `backend/internal/domain/period_test.go`
```go
package domain_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

func moscow(t *testing.T) *time.Location {
	t.Helper()
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)
	return loc
}

func TestResolvePeriod(t *testing.T) {
	// Thursday 2026-10-15, 12:00 Moscow
	now := time.Date(2026, 10, 15, 12, 0, 0, 0, moscow(t))
	cases := []struct {
		p        domain.Period
		from, to string
	}{
		{domain.PeriodToday, "2026-10-15", "2026-10-15"},
		{domain.PeriodYesterday, "2026-10-14", "2026-10-14"},
		{domain.PeriodThisWeek, "2026-10-12", "2026-10-15"},
		{domain.PeriodLastWeek, "2026-10-05", "2026-10-11"},
		{domain.PeriodThisMonth, "2026-10-01", "2026-10-15"},
		{domain.PeriodLastMonth, "2026-09-01", "2026-09-30"},
		{domain.PeriodLast7Days, "2026-10-09", "2026-10-15"},
		{domain.PeriodLast30Days, "2026-09-16", "2026-10-15"},
	}
	for _, c := range cases {
		t.Run(string(c.p), func(t *testing.T) {
			from, to, err := domain.ResolvePeriod(c.p, now)
			require.NoError(t, err)
			assert.Equal(t, c.from, domain.FormatDate(from))
			assert.Equal(t, c.to, domain.FormatDate(to))
		})
	}
}

func TestResolvePeriodWeekEdges(t *testing.T) {
	loc := moscow(t)
	sunday := time.Date(2026, 10, 18, 10, 0, 0, 0, loc)
	from, to, err := domain.ResolvePeriod(domain.PeriodThisWeek, sunday)
	require.NoError(t, err)
	assert.Equal(t, "2026-10-12", domain.FormatDate(from))
	assert.Equal(t, "2026-10-18", domain.FormatDate(to))

	monday := time.Date(2026, 10, 12, 10, 0, 0, 0, loc)
	from, to, err = domain.ResolvePeriod(domain.PeriodThisWeek, monday)
	require.NoError(t, err)
	assert.Equal(t, "2026-10-12", domain.FormatDate(from))
	assert.Equal(t, "2026-10-12", domain.FormatDate(to))
}

func TestResolvePeriodLastMonthInJanuary(t *testing.T) {
	now := time.Date(2026, 1, 10, 10, 0, 0, 0, moscow(t))
	from, to, err := domain.ResolvePeriod(domain.PeriodLastMonth, now)
	require.NoError(t, err)
	assert.Equal(t, "2025-12-01", domain.FormatDate(from))
	assert.Equal(t, "2025-12-31", domain.FormatDate(to))
}

func TestResolvePeriodUnknown(t *testing.T) {
	_, _, err := domain.ResolvePeriod("fortnight", time.Now())
	assert.Error(t, err)
}

func TestDateOfUsesTimeLocation(t *testing.T) {
	// 22:30 UTC is already the next day in Moscow (UTC+3).
	utc := time.Date(2026, 10, 15, 22, 30, 0, 0, time.UTC)
	assert.Equal(t, "2026-10-16", domain.FormatDate(domain.DateOf(utc.In(moscow(t)))))
	assert.Equal(t, time.UTC, domain.DateOf(utc).Location())
}

func TestParseDate(t *testing.T) {
	d, err := domain.ParseDate("2026-10-14")
	require.NoError(t, err)
	assert.Equal(t, "2026-10-14", domain.FormatDate(d))
	assert.Equal(t, "среда", domain.WeekdayRU(d))

	_, err = domain.ParseDate("14.10.2026")
	assert.Error(t, err)
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/domain/`
Expected: FAIL — undefined `domain.ParseCategory`, `domain.ResolvePeriod`, etc.

- [ ] **Step 3: Implement**

**File:** `backend/internal/domain/errors.go`
```go
package domain

import "errors"

// ErrNotFound is returned when an entity does not exist or is outside the caller's scope.
var ErrNotFound = errors.New("не найдено")

// ValidationError describes invalid input for a single field.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string { return e.Field + ": " + e.Message }

// Invalid builds a *ValidationError.
func Invalid(field, message string) error {
	return &ValidationError{Field: field, Message: message}
}
```

**File:** `backend/internal/domain/category.go`
```go
package domain

import (
	"fmt"
	"strings"
)

// Category is an expense category code stored in the database and API.
type Category string

const (
	CategoryFood          Category = "food"
	CategoryTransport     Category = "transport"
	CategoryHousing       Category = "housing"
	CategoryEntertainment Category = "entertainment"
	CategoryHealth        Category = "health"
	CategoryShopping      Category = "shopping"
	CategoryOther         Category = "other"
)

// Categories lists all categories in display order.
var Categories = []Category{
	CategoryFood, CategoryTransport, CategoryHousing, CategoryEntertainment,
	CategoryHealth, CategoryShopping, CategoryOther,
}

var categoryNames = map[Category]string{
	CategoryFood:          "еда",
	CategoryTransport:     "транспорт",
	CategoryHousing:       "жильё",
	CategoryEntertainment: "развлечения",
	CategoryHealth:        "здоровье",
	CategoryShopping:      "покупки",
	CategoryOther:         "другое",
}

// Valid reports whether c is a known category.
func (c Category) Valid() bool {
	_, ok := categoryNames[c]
	return ok
}

// RU returns the Russian display name.
func (c Category) RU() string {
	if name, ok := categoryNames[c]; ok {
		return name
	}
	return string(c)
}

// ParseCategory accepts a code ("food") or a Russian name ("еда", "жилье").
func ParseCategory(s string) (Category, error) {
	v := normalizeYo(strings.ToLower(strings.TrimSpace(s)))
	for c, name := range categoryNames {
		if v == string(c) || v == normalizeYo(name) {
			return c, nil
		}
	}
	return "", Invalid("category", fmt.Sprintf("неизвестная категория %q", s))
}

func normalizeYo(s string) string { return strings.ReplaceAll(s, "ё", "е") }
```

**File:** `backend/internal/domain/date.go`
```go
package domain

import (
	"fmt"
	"strings"
	"time"
)

// DateLayout is the ISO date format used in the API and agent tools.
const DateLayout = "2006-01-02"

// DateOf returns midnight UTC of t's calendar day in t's own location.
// Convert t to the user's location first: DateOf(now.In(loc)).
func DateOf(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// ParseDate parses "YYYY-MM-DD" into a domain date.
func ParseDate(s string) (time.Time, error) {
	t, err := time.Parse(DateLayout, strings.TrimSpace(s))
	if err != nil {
		return time.Time{}, Invalid("date", fmt.Sprintf("дата %q не в формате ГГГГ-ММ-ДД", s))
	}
	return t, nil
}

// FormatDate renders a domain date as "YYYY-MM-DD".
func FormatDate(t time.Time) string { return t.Format(DateLayout) }

var weekdaysRU = [...]string{"воскресенье", "понедельник", "вторник", "среда", "четверг", "пятница", "суббота"}

// WeekdayRU returns the Russian weekday name.
func WeekdayRU(t time.Time) string { return weekdaysRU[t.Weekday()] }
```

**File:** `backend/internal/domain/period.go`
```go
package domain

import (
	"fmt"
	"time"
)

// Period is a named relative date range the agent can ask for.
type Period string

const (
	PeriodToday      Period = "today"
	PeriodYesterday  Period = "yesterday"
	PeriodThisWeek   Period = "this_week"
	PeriodLastWeek   Period = "last_week"
	PeriodThisMonth  Period = "this_month"
	PeriodLastMonth  Period = "last_month"
	PeriodLast7Days  Period = "last_7_days"
	PeriodLast30Days Period = "last_30_days"
)

// Periods lists all supported periods.
var Periods = []Period{
	PeriodToday, PeriodYesterday, PeriodThisWeek, PeriodLastWeek,
	PeriodThisMonth, PeriodLastMonth, PeriodLast7Days, PeriodLast30Days,
}

// ResolvePeriod returns the inclusive [from, to] domain dates of p relative to now.
// now must already be in the user's location. Weeks start on Monday.
func ResolvePeriod(p Period, now time.Time) (from, to time.Time, err error) {
	today := DateOf(now)
	sinceMonday := (int(today.Weekday()) + 6) % 7
	monday := today.AddDate(0, 0, -sinceMonday)
	firstOfMonth := time.Date(today.Year(), today.Month(), 1, 0, 0, 0, 0, time.UTC)

	switch p {
	case PeriodToday:
		return today, today, nil
	case PeriodYesterday:
		y := today.AddDate(0, 0, -1)
		return y, y, nil
	case PeriodThisWeek:
		return monday, today, nil
	case PeriodLastWeek:
		return monday.AddDate(0, 0, -7), monday.AddDate(0, 0, -1), nil
	case PeriodThisMonth:
		return firstOfMonth, today, nil
	case PeriodLastMonth:
		return firstOfMonth.AddDate(0, -1, 0), firstOfMonth.AddDate(0, 0, -1), nil
	case PeriodLast7Days:
		return today.AddDate(0, 0, -6), today, nil
	case PeriodLast30Days:
		return today.AddDate(0, 0, -29), today, nil
	}
	return time.Time{}, time.Time{}, Invalid("period", fmt.Sprintf("неизвестный период %q", p))
}
```

**File:** `backend/internal/domain/types.go`
```go
package domain

import (
	"encoding/json"
	"time"
)

// Expense is a single spending record owned by a family member.
type Expense struct {
	ID         int64
	UserID     int64
	MemberName string
	Amount     Money
	Category   Category
	SpentAt    time.Time // domain date, see DateOf
	Comment    string    // empty means no comment
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Member is a family member allowed to use the bot.
type Member struct {
	TelegramID int64
	Name       string
}

// Trace records one agent run for the panel's agent journal.
type Trace struct {
	ID               int64
	UserID           int64
	MemberName       string
	Source           string // "text" | "voice"
	Input            string
	Output           string // empty means none
	Steps            []TraceStep
	Model            string
	PromptTokens     int
	CompletionTokens int
	STTms            *int
	LatencyMs        int
	Error            string // empty means none
	CreatedAt        time.Time
}

// TraceStep is one tool call made by the agent.
type TraceStep struct {
	Tool   string          `json:"tool"`
	Args   json.RawMessage `json:"args"`
	Result json.RawMessage `json:"result"`
	Ms     int64           `json:"ms"`
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test ./internal/domain/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/domain
git commit -m "feat(domain): add categories, dates, periods and core types"
```

---

### Task 3: Configuration

**Files:**
- Create: `backend/internal/config/config.go`
- Test: `backend/internal/config/config_test.go`

**Interfaces:**
- Consumes: `domain.Member`.
- Produces: `type config.Config` (fields below); `func config.Load() (Config, error)`; `func config.LoadEnv(environ map[string]string) (Config, error)`; `func config.ParseMembers(s string) ([]domain.Member, error)`. Config fields: `TelegramToken, FamilyMembers, LLMAPIKey, LLMBaseURL, LLMModel string; LLMTimeout time.Duration; STTEnabled bool; STTModel string; STTMaxSeconds int; Timezone, DatabaseURL, HTTPAddr, LogLevel string; SeedDemo bool; Members []domain.Member; Location *time.Location`.

- [ ] **Step 1: Add dependency**

```bash
cd backend && go get github.com/caarlos0/env/v11@v11.4.1
```

- [ ] **Step 2: Write the failing test**

**File:** `backend/internal/config/config_test.go`
```go
package config_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/config"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

func baseEnv() map[string]string {
	return map[string]string{
		"TELEGRAM_TOKEN": "tg-token",
		"FAMILY_MEMBERS": "111:Папа, 222:Мама",
		"LLM_API_KEY":    "key",
		"LLM_MODEL":      "anthropic/claude-haiku-5.5",
		"DATABASE_URL":   "postgres://x",
	}
}

func TestLoadDefaults(t *testing.T) {
	cfg, err := config.LoadEnv(baseEnv())
	require.NoError(t, err)
	assert.Equal(t, "https://openrouter.ai/api/v1", cfg.LLMBaseURL)
	assert.Equal(t, 30*time.Second, cfg.LLMTimeout)
	assert.True(t, cfg.STTEnabled)
	assert.Equal(t, "openai/whisper-large-v3", cfg.STTModel)
	assert.Equal(t, 60, cfg.STTMaxSeconds)
	assert.Equal(t, ":8080", cfg.HTTPAddr)
	assert.Equal(t, "Europe/Moscow", cfg.Location.String())
	assert.False(t, cfg.SeedDemo)
	assert.Equal(t, []domain.Member{{TelegramID: 111, Name: "Папа"}, {TelegramID: 222, Name: "Мама"}}, cfg.Members)
}

func TestLoadMissingRequired(t *testing.T) {
	env := baseEnv()
	delete(env, "LLM_MODEL")
	_, err := config.LoadEnv(env)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "LLM_MODEL")
}

func TestLoadBadTimezone(t *testing.T) {
	env := baseEnv()
	env["APP_TIMEZONE"] = "Mars/Olympus"
	_, err := config.LoadEnv(env)
	assert.ErrorContains(t, err, "APP_TIMEZONE")
}

func TestParseMembersErrors(t *testing.T) {
	for _, in := range []string{"", "abc", "111", "111:", ":Папа", "0:Папа", "111:A,111:B"} {
		_, err := config.ParseMembers(in)
		assert.Error(t, err, "input %q", in)
	}
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd backend && go test ./internal/config/`
Expected: FAIL — package `config` has no non-test files / undefined `config.LoadEnv`.

- [ ] **Step 4: Implement**

**File:** `backend/internal/config/config.go`
```go
// Package config loads service configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	_ "time/tzdata" // embed the tz database: distroless images ship none

	"github.com/caarlos0/env/v11"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

// Config is the full service configuration.
type Config struct {
	TelegramToken string        `env:"TELEGRAM_TOKEN,required"`
	FamilyMembers string        `env:"FAMILY_MEMBERS,required"`
	LLMAPIKey     string        `env:"LLM_API_KEY,required"`
	LLMBaseURL    string        `env:"LLM_BASE_URL" envDefault:"https://openrouter.ai/api/v1"`
	LLMModel      string        `env:"LLM_MODEL,required"`
	LLMTimeout    time.Duration `env:"LLM_TIMEOUT" envDefault:"30s"`
	STTEnabled    bool          `env:"STT_ENABLED" envDefault:"true"`
	STTModel      string        `env:"STT_MODEL" envDefault:"openai/whisper-large-v3"`
	STTMaxSeconds int           `env:"STT_MAX_SECONDS" envDefault:"60"`
	Timezone      string        `env:"APP_TIMEZONE" envDefault:"Europe/Moscow"`
	DatabaseURL   string        `env:"DATABASE_URL,required"`
	HTTPAddr      string        `env:"HTTP_ADDR" envDefault:":8080"`
	LogLevel      string        `env:"LOG_LEVEL" envDefault:"info"`
	SeedDemo      bool          `env:"SEED_DEMO" envDefault:"false"`

	// Derived values, filled by Load.
	Members  []domain.Member
	Location *time.Location
}

// Load reads configuration from the process environment.
func Load() (Config, error) {
	cfg, err := env.ParseAs[Config]()
	if err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}
	return finalize(cfg)
}

// LoadEnv reads configuration from the given map instead of the process environment.
func LoadEnv(environ map[string]string) (Config, error) {
	cfg, err := env.ParseAsWithOptions[Config](env.Options{Environment: environ})
	if err != nil {
		return Config{}, fmt.Errorf("parse env: %w", err)
	}
	return finalize(cfg)
}

func finalize(cfg Config) (Config, error) {
	members, err := ParseMembers(cfg.FamilyMembers)
	if err != nil {
		return Config{}, fmt.Errorf("FAMILY_MEMBERS: %w", err)
	}
	cfg.Members = members

	loc, err := time.LoadLocation(cfg.Timezone)
	if err != nil {
		return Config{}, fmt.Errorf("APP_TIMEZONE: %w", err)
	}
	cfg.Location = loc

	if cfg.STTMaxSeconds <= 0 {
		return Config{}, errors.New("STT_MAX_SECONDS must be positive")
	}
	return cfg, nil
}

// ParseMembers parses "111:Папа, 222:Мама".
func ParseMembers(s string) ([]domain.Member, error) {
	var out []domain.Member
	seen := map[int64]bool{}
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		idStr, name, ok := strings.Cut(part, ":")
		if !ok {
			return nil, fmt.Errorf("%q: ожидается формат ID:Имя", part)
		}
		id, err := strconv.ParseInt(strings.TrimSpace(idStr), 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("%q: некорректный Telegram ID", part)
		}
		name = strings.TrimSpace(name)
		if name == "" {
			return nil, fmt.Errorf("%q: пустое имя", part)
		}
		if seen[id] {
			return nil, fmt.Errorf("повторяется Telegram ID %d", id)
		}
		seen[id] = true
		out = append(out, domain.Member{TelegramID: id, Name: name})
	}
	if len(out) == 0 {
		return nil, errors.New("список членов семьи пуст")
	}
	return out, nil
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd backend && go test ./internal/config/`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add backend/go.mod backend/go.sum backend/internal/config
git commit -m "feat(config): load env configuration and family members"
```

---

### Task 4: Repository interface, contract suite, in-memory repository

**Files:**
- Create: `backend/internal/expense/repository.go`
- Create: `backend/internal/expense/expensetest/contract.go`
- Create: `backend/internal/expense/memrepo/memrepo.go`
- Test: `backend/internal/expense/memrepo/memrepo_test.go`

**Interfaces:**
- Consumes: `domain.Expense`, `domain.Member`, `domain.Money`, `domain.Category`, `domain.ErrNotFound`, `domain.DateLayout`, `domain.FormatDate`.
- Produces:
  - `type expense.ListQuery struct{ UserID *int64; From, To *time.Time; Category *domain.Category; Text string; SortRecent bool; Limit, Offset int }`
  - `type expense.AggQuery struct{ From, To time.Time; UserID *int64; Category *domain.Category }`
  - `type expense.AggRow struct{ Date time.Time; UserID int64; MemberName string; Category domain.Category; Total domain.Money; Count int }`
  - `type expense.Repository interface` (methods below)
  - `expensetest.Papa` (111, "Папа"), `expensetest.Mama` (222, "Мама"), `func expensetest.Date(s string) time.Time`, `type expensetest.Factory func(t *testing.T) expense.Repository`, `func expensetest.RunRepositoryContract(t *testing.T, newRepo Factory)`
  - `func memrepo.New() *memrepo.Repo`; `func (*Repo) Snapshot() []domain.Expense` (ordered by ID)

- [ ] **Step 1: Write the interface (no behaviour yet, compiles on its own)**

**File:** `backend/internal/expense/repository.go`
```go
// Package expense holds the business rules for family expenses.
package expense

import (
	"context"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

// ListQuery filters and pages expenses. Nil pointers mean "no filter".
type ListQuery struct {
	UserID     *int64
	From, To   *time.Time // inclusive domain dates
	Category   *domain.Category
	Text       string // case-insensitive substring of the comment
	SortRecent bool   // newest created first, instead of newest spent first
	Limit      int    // <= 0 means no limit (Service always sets one)
	Offset     int
}

// AggQuery selects expenses for aggregation over an inclusive date range.
type AggQuery struct {
	From, To time.Time
	UserID   *int64
	Category *domain.Category
}

// AggRow is a total grouped by (date, user, category).
type AggRow struct {
	Date       time.Time
	UserID     int64
	MemberName string
	Category   domain.Category
	Total      domain.Money
	Count      int
}

// Repository persists expenses and members.
// Implementations: store/postgres (production) and expense/memrepo (tests, evals),
// both verified by expensetest.RunRepositoryContract.
type Repository interface {
	UpsertMembers(ctx context.Context, members []domain.Member) error
	// ListMembers returns members ordered by name.
	ListMembers(ctx context.Context) ([]domain.Member, error)
	MemberExists(ctx context.Context, telegramID int64) (bool, error)

	// Insert stores e and fills ID, MemberName, CreatedAt and UpdatedAt.
	Insert(ctx context.Context, e domain.Expense) (domain.Expense, error)
	// Get returns domain.ErrNotFound if the expense is missing or owner != nil and it belongs to someone else.
	Get(ctx context.Context, id int64, owner *int64) (domain.Expense, error)
	// List orders by created_at desc (only if SortRecent), then spent_at desc, then id desc.
	List(ctx context.Context, q ListQuery) ([]domain.Expense, error)
	// Count returns the number and sum of expenses matching q, ignoring Limit and Offset.
	Count(ctx context.Context, q ListQuery) (int, domain.Money, error)
	// Update replaces all mutable fields of the expense with ID e.ID and refreshes UpdatedAt.
	Update(ctx context.Context, e domain.Expense) (domain.Expense, error)
	// Delete returns domain.ErrNotFound if the expense is missing.
	Delete(ctx context.Context, id int64) error
	// Aggregate groups by (date, user, category) and orders rows the same way.
	Aggregate(ctx context.Context, q AggQuery) ([]AggRow, error)
}
```

- [ ] **Step 2: Write the contract suite (the failing tests)**

**File:** `backend/internal/expense/expensetest/contract.go`
```go
// Package expensetest provides a shared behavioural contract for expense.Repository implementations.
package expensetest

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
)

// Test family.
var (
	Papa = domain.Member{TelegramID: 111, Name: "Папа"}
	Mama = domain.Member{TelegramID: 222, Name: "Мама"}
)

// Factory returns an empty repository.
type Factory func(t *testing.T) expense.Repository

// Date parses "YYYY-MM-DD" and panics on error (test helper).
func Date(s string) time.Time {
	t, err := time.Parse(domain.DateLayout, s)
	if err != nil {
		panic(err)
	}
	return t
}

func ptr[T any](v T) *T { return &v }

type aggKey struct {
	Date     string
	UserID   int64
	Member   string
	Category domain.Category
	Total    domain.Money
	Count    int
}

// RunRepositoryContract runs the behaviour every Repository implementation must satisfy.
func RunRepositoryContract(t *testing.T, newRepo Factory) {
	ctx := context.Background()

	setup := func(t *testing.T) expense.Repository {
		t.Helper()
		r := newRepo(t)
		require.NoError(t, r.UpsertMembers(ctx, []domain.Member{Papa, Mama}))
		return r
	}
	insert := func(t *testing.T, r expense.Repository, user int64, amount domain.Money, cat domain.Category, date, comment string) domain.Expense {
		t.Helper()
		e, err := r.Insert(ctx, domain.Expense{UserID: user, Amount: amount, Category: cat, SpentAt: Date(date), Comment: comment})
		require.NoError(t, err)
		return e
	}
	// fixture inserts e1..e4 in this order.
	fixture := func(t *testing.T, r expense.Repository) (e1, e2, e3, e4 domain.Expense) {
		t.Helper()
		e1 = insert(t, r, Papa.TelegramID, 35000, domain.CategoryFood, "2026-10-10", "кофе")
		e2 = insert(t, r, Papa.TelegramID, 120000, domain.CategoryTransport, "2026-10-14", "такси")
		e3 = insert(t, r, Mama.TelegramID, 500000, domain.CategoryShopping, "2026-10-12", "кроссовки")
		e4 = insert(t, r, Papa.TelegramID, 90000, domain.CategoryTransport, "2026-10-14", "Такси домой")
		return
	}
	ids := func(items []domain.Expense) []int64 {
		out := make([]int64, 0, len(items))
		for _, e := range items {
			out = append(out, e.ID)
		}
		return out
	}
	list := func(t *testing.T, r expense.Repository, q expense.ListQuery) []int64 {
		t.Helper()
		if q.Limit == 0 {
			q.Limit = 50
		}
		items, err := r.List(ctx, q)
		require.NoError(t, err)
		return ids(items)
	}

	t.Run("members", func(t *testing.T) {
		r := setup(t)
		members, err := r.ListMembers(ctx)
		require.NoError(t, err)
		assert.Equal(t, []domain.Member{Mama, Papa}, members)

		renamed := domain.Member{TelegramID: Papa.TelegramID, Name: "Папа Вова"}
		require.NoError(t, r.UpsertMembers(ctx, []domain.Member{renamed}))
		members, err = r.ListMembers(ctx)
		require.NoError(t, err)
		assert.Equal(t, []domain.Member{Mama, renamed}, members)

		ok, err := r.MemberExists(ctx, Papa.TelegramID)
		require.NoError(t, err)
		assert.True(t, ok)
		ok, err = r.MemberExists(ctx, 999)
		require.NoError(t, err)
		assert.False(t, ok)
	})

	t.Run("insert and get", func(t *testing.T) {
		r := setup(t)
		e := insert(t, r, Papa.TelegramID, 35000, domain.CategoryFood, "2026-10-10", "кофе")
		assert.Positive(t, e.ID)
		assert.Equal(t, "Папа", e.MemberName)
		assert.Equal(t, domain.Money(35000), e.Amount)
		assert.Equal(t, domain.CategoryFood, e.Category)
		assert.Equal(t, "2026-10-10", domain.FormatDate(e.SpentAt))
		assert.Equal(t, "кофе", e.Comment)
		assert.False(t, e.CreatedAt.IsZero())
		assert.True(t, e.UpdatedAt.Equal(e.CreatedAt))

		got, err := r.Get(ctx, e.ID, nil)
		require.NoError(t, err)
		assert.Equal(t, e.ID, got.ID)
		assert.Equal(t, e.Amount, got.Amount)
		assert.Equal(t, "Папа", got.MemberName)
		assert.Equal(t, "2026-10-10", domain.FormatDate(got.SpentAt))
		assert.Equal(t, "кофе", got.Comment)
	})

	t.Run("get respects owner", func(t *testing.T) {
		r := setup(t)
		e := insert(t, r, Papa.TelegramID, 100, domain.CategoryOther, "2026-10-10", "")

		_, err := r.Get(ctx, e.ID, ptr(Mama.TelegramID))
		assert.ErrorIs(t, err, domain.ErrNotFound)
		_, err = r.Get(ctx, e.ID, ptr(Papa.TelegramID))
		assert.NoError(t, err)
		_, err = r.Get(ctx, 9999, nil)
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("list filters and order", func(t *testing.T) {
		r := setup(t)
		e1, e2, e3, e4 := fixture(t, r)

		assert.Equal(t, []int64{e4.ID, e2.ID, e3.ID, e1.ID}, list(t, r, expense.ListQuery{}))
		assert.Equal(t, []int64{e4.ID, e2.ID, e1.ID}, list(t, r, expense.ListQuery{UserID: ptr(Papa.TelegramID)}))
		assert.Equal(t, []int64{e3.ID}, list(t, r, expense.ListQuery{From: ptr(Date("2026-10-12")), To: ptr(Date("2026-10-13"))}))
		assert.Equal(t, []int64{e4.ID, e2.ID}, list(t, r, expense.ListQuery{Category: ptr(domain.CategoryTransport)}))
		// Case-insensitive Cyrillic search.
		assert.Equal(t, []int64{e4.ID, e2.ID}, list(t, r, expense.ListQuery{Text: "такси"}))
		assert.Equal(t, []int64{e4.ID, e2.ID}, list(t, r, expense.ListQuery{Text: "ТАКСИ"}))
		assert.Equal(t, []int64{e4.ID, e3.ID, e2.ID, e1.ID}, list(t, r, expense.ListQuery{SortRecent: true}))
		assert.Equal(t, []int64{e2.ID, e3.ID}, list(t, r, expense.ListQuery{Limit: 2, Offset: 1}))
		assert.Empty(t, list(t, r, expense.ListQuery{Offset: 10}))
	})

	t.Run("count ignores pagination", func(t *testing.T) {
		r := setup(t)
		fixture(t, r)

		n, sum, err := r.Count(ctx, expense.ListQuery{Limit: 1, Offset: 3})
		require.NoError(t, err)
		assert.Equal(t, 4, n)
		assert.Equal(t, domain.Money(745000), sum)

		n, sum, err = r.Count(ctx, expense.ListQuery{Text: "такси"})
		require.NoError(t, err)
		assert.Equal(t, 2, n)
		assert.Equal(t, domain.Money(210000), sum)

		n, sum, err = r.Count(ctx, expense.ListQuery{UserID: ptr(Mama.TelegramID)})
		require.NoError(t, err)
		assert.Equal(t, 1, n)
		assert.Equal(t, domain.Money(500000), sum)

		// LIKE wildcards in user text are literal.
		n, sum, err = r.Count(ctx, expense.ListQuery{Text: "%"})
		require.NoError(t, err)
		assert.Equal(t, 0, n)
		assert.Equal(t, domain.Money(0), sum)
	})

	t.Run("update", func(t *testing.T) {
		r := setup(t)
		e := insert(t, r, Papa.TelegramID, 35000, domain.CategoryFood, "2026-10-10", "кофе")

		e.Amount = 45000
		e.Category = domain.CategoryOther
		e.SpentAt = Date("2026-10-11")
		e.Comment = "обед"
		e.UserID = Mama.TelegramID
		upd, err := r.Update(ctx, e)
		require.NoError(t, err)
		assert.Equal(t, domain.Money(45000), upd.Amount)
		assert.Equal(t, domain.CategoryOther, upd.Category)
		assert.Equal(t, "2026-10-11", domain.FormatDate(upd.SpentAt))
		assert.Equal(t, "обед", upd.Comment)
		assert.Equal(t, Mama.TelegramID, upd.UserID)
		assert.Equal(t, "Мама", upd.MemberName)
		assert.False(t, upd.UpdatedAt.Before(upd.CreatedAt))

		got, err := r.Get(ctx, e.ID, nil)
		require.NoError(t, err)
		assert.Equal(t, domain.Money(45000), got.Amount)

		_, err = r.Update(ctx, domain.Expense{ID: 9999, UserID: Papa.TelegramID, Amount: 1, Category: domain.CategoryFood, SpentAt: Date("2026-10-10")})
		assert.ErrorIs(t, err, domain.ErrNotFound)
	})

	t.Run("delete", func(t *testing.T) {
		r := setup(t)
		e := insert(t, r, Papa.TelegramID, 100, domain.CategoryOther, "2026-10-10", "")

		require.NoError(t, r.Delete(ctx, e.ID))
		_, err := r.Get(ctx, e.ID, nil)
		assert.ErrorIs(t, err, domain.ErrNotFound)
		assert.ErrorIs(t, r.Delete(ctx, e.ID), domain.ErrNotFound)
	})

	t.Run("aggregate", func(t *testing.T) {
		r := setup(t)
		fixture(t, r)
		agg := func(q expense.AggQuery) []aggKey {
			t.Helper()
			rows, err := r.Aggregate(ctx, q)
			require.NoError(t, err)
			out := make([]aggKey, 0, len(rows))
			for _, row := range rows {
				out = append(out, aggKey{domain.FormatDate(row.Date), row.UserID, row.MemberName, row.Category, row.Total, row.Count})
			}
			return out
		}

		all := expense.AggQuery{From: Date("2026-10-10"), To: Date("2026-10-14")}
		assert.Equal(t, []aggKey{
			{"2026-10-10", Papa.TelegramID, "Папа", domain.CategoryFood, 35000, 1},
			{"2026-10-12", Mama.TelegramID, "Мама", domain.CategoryShopping, 500000, 1},
			{"2026-10-14", Papa.TelegramID, "Папа", domain.CategoryTransport, 210000, 2},
		}, agg(all))

		papaOnly := all
		papaOnly.UserID = ptr(Papa.TelegramID)
		assert.Len(t, agg(papaOnly), 2)

		transport := all
		transport.Category = ptr(domain.CategoryTransport)
		assert.Len(t, agg(transport), 1)

		assert.Len(t, agg(expense.AggQuery{From: Date("2026-10-11"), To: Date("2026-10-13")}), 1)
	})
}
```

**File:** `backend/internal/expense/memrepo/memrepo_test.go`
```go
package memrepo_test

import (
	"testing"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/expensetest"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/memrepo"
)

func TestRepositoryContract(t *testing.T) {
	expensetest.RunRepositoryContract(t, func(*testing.T) expense.Repository { return memrepo.New() })
}
```

- [ ] **Step 3: Run test to verify it fails**

Run: `cd backend && go test ./internal/expense/...`
Expected: FAIL — `undefined: memrepo.New`.

- [ ] **Step 4: Implement the in-memory repository**

**File:** `backend/internal/expense/memrepo/memrepo.go`
```go
// Package memrepo is an in-memory expense.Repository for unit tests and agent evals.
package memrepo

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
)

// Repo is safe for concurrent use.
type Repo struct {
	mu      sync.Mutex
	members map[int64]string
	items   map[int64]domain.Expense
	nextID  int64
	last    time.Time
}

var _ expense.Repository = (*Repo)(nil)

// New returns an empty repository.
func New() *Repo {
	return &Repo{members: map[int64]string{}, items: map[int64]domain.Expense{}}
}

// stamp returns a strictly increasing timestamp so ordering by creation time is deterministic.
func (r *Repo) stamp() time.Time {
	now := time.Now().UTC()
	if !now.After(r.last) {
		now = r.last.Add(time.Microsecond)
	}
	r.last = now
	return now
}

func (r *Repo) UpsertMembers(_ context.Context, members []domain.Member) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range members {
		r.members[m.TelegramID] = m.Name
	}
	return nil
}

func (r *Repo) ListMembers(_ context.Context) ([]domain.Member, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Member, 0, len(r.members))
	for id, name := range r.members {
		out = append(out, domain.Member{TelegramID: id, Name: name})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].TelegramID < out[j].TelegramID
	})
	return out, nil
}

func (r *Repo) MemberExists(_ context.Context, id int64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.members[id]
	return ok, nil
}

func (r *Repo) Insert(_ context.Context, e domain.Expense) (domain.Expense, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name, ok := r.members[e.UserID]
	if !ok {
		return domain.Expense{}, fmt.Errorf("member %d does not exist", e.UserID)
	}
	r.nextID++
	e.ID = r.nextID
	e.MemberName = name
	e.CreatedAt = r.stamp()
	e.UpdatedAt = e.CreatedAt
	r.items[e.ID] = e
	return e, nil
}

func (r *Repo) Get(_ context.Context, id int64, owner *int64) (domain.Expense, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.items[id]
	if !ok || (owner != nil && e.UserID != *owner) {
		return domain.Expense{}, domain.ErrNotFound
	}
	e.MemberName = r.members[e.UserID]
	return e, nil
}

func (r *Repo) List(_ context.Context, q expense.ListQuery) ([]domain.Expense, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.filter(q)
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if q.SortRecent && !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.After(b.CreatedAt)
		}
		if !a.SpentAt.Equal(b.SpentAt) {
			return a.SpentAt.After(b.SpentAt)
		}
		return a.ID > b.ID
	})
	if q.Offset >= len(out) {
		return []domain.Expense{}, nil
	}
	out = out[q.Offset:]
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

func (r *Repo) Count(_ context.Context, q expense.ListQuery) (int, domain.Money, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var sum domain.Money
	matched := r.filter(q)
	for _, e := range matched {
		sum += e.Amount
	}
	return len(matched), sum, nil
}

func (r *Repo) Update(_ context.Context, e domain.Expense) (domain.Expense, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.items[e.ID]
	if !ok {
		return domain.Expense{}, domain.ErrNotFound
	}
	name, ok := r.members[e.UserID]
	if !ok {
		return domain.Expense{}, fmt.Errorf("member %d does not exist", e.UserID)
	}
	e.CreatedAt = cur.CreatedAt
	e.UpdatedAt = r.stamp()
	e.MemberName = name
	r.items[e.ID] = e
	return e, nil
}

func (r *Repo) Delete(_ context.Context, id int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.items[id]; !ok {
		return domain.ErrNotFound
	}
	delete(r.items, id)
	return nil
}

func (r *Repo) Aggregate(_ context.Context, q expense.AggQuery) ([]expense.AggRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	type key struct {
		date time.Time
		user int64
		cat  domain.Category
	}
	groups := map[key]*expense.AggRow{}
	for _, e := range r.filter(expense.ListQuery{UserID: q.UserID, From: &q.From, To: &q.To, Category: q.Category}) {
		k := key{e.SpentAt, e.UserID, e.Category}
		g, ok := groups[k]
		if !ok {
			g = &expense.AggRow{Date: e.SpentAt, UserID: e.UserID, MemberName: e.MemberName, Category: e.Category}
			groups[k] = g
		}
		g.Total += e.Amount
		g.Count++
	}
	out := make([]expense.AggRow, 0, len(groups))
	for _, g := range groups {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if !a.Date.Equal(b.Date) {
			return a.Date.Before(b.Date)
		}
		if a.UserID != b.UserID {
			return a.UserID < b.UserID
		}
		return a.Category < b.Category
	})
	return out, nil
}

// Snapshot returns all expenses ordered by ID. Used by evals to diff state.
func (r *Repo) Snapshot() []domain.Expense {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Expense, 0, len(r.items))
	for _, e := range r.items {
		e.MemberName = r.members[e.UserID]
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// filter must be called with r.mu held.
func (r *Repo) filter(q expense.ListQuery) []domain.Expense {
	text := strings.ToLower(q.Text)
	var out []domain.Expense
	for _, e := range r.items {
		switch {
		case q.UserID != nil && e.UserID != *q.UserID,
			q.From != nil && e.SpentAt.Before(*q.From),
			q.To != nil && e.SpentAt.After(*q.To),
			q.Category != nil && e.Category != *q.Category,
			text != "" && !strings.Contains(strings.ToLower(e.Comment), text):
			continue
		}
		e.MemberName = r.members[e.UserID]
		out = append(out, e)
	}
	return out
}
```

- [ ] **Step 5: Run tests to verify they pass**

Run: `cd backend && go test ./internal/expense/...`
Expected: PASS (all `TestRepositoryContract/*` subtests).

- [ ] **Step 6: Commit**

```bash
git add backend/internal/expense
git commit -m "feat(expense): add repository contract and in-memory repository"
```

---

### Task 5: Postgres store (migrations, sqlc, repository, agent storage)

**Files:**
- Create: `backend/migrations/00001_init.sql`, `backend/migrations/embed.go`
- Create: `backend/sqlc.yaml`
- Create: `backend/internal/store/postgres/queries/members.sql`, `expenses.sql`, `agent.sql`
- Generate: `backend/internal/store/postgres/db/*` (via `make gen-sql`)
- Create: `backend/internal/store/postgres/postgres.go`, `members.go`, `expenses.go`, `agent.go`
- Create: `backend/internal/testutil/pgtest/pgtest.go`
- Test: `backend/internal/store/postgres/postgres_test.go`

**Interfaces:**
- Consumes: `expense.Repository`, `expense.ListQuery`, `expense.AggQuery`, `expense.AggRow`, `expensetest.RunRepositoryContract`, `expensetest.Papa/Mama`, `domain.Trace`, `domain.TraceStep`.
- Produces:
  - `func postgres.Open(ctx context.Context, url string) (*pgxpool.Pool, error)`; `func postgres.Migrate(ctx context.Context, pool *pgxpool.Pool) error`
  - `type postgres.Store`; `func postgres.New(pool *pgxpool.Pool) *postgres.Store` — implements `expense.Repository` plus:
    - `AppendChat(ctx context.Context, userID int64, msgs []json.RawMessage) error`
    - `RecentChat(ctx context.Context, userID int64, since time.Time, limit int) ([]json.RawMessage, error)` — chronological order, the latest `limit` messages not older than `since`
    - `SaveTrace(ctx context.Context, t domain.Trace) error`
    - `ListTraces(ctx context.Context, userID *int64, limit, offset int) ([]domain.Trace, int, error)` — newest first, plus total count
  - `func pgtest.Pool(t *testing.T) *pgxpool.Pool` — shared migrated testcontainer, tables truncated per call; skips under `-short`.

- [ ] **Step 1: Add dependencies**

```bash
cd backend
go get github.com/jackc/pgx/v5@v5.11.0 github.com/pressly/goose/v3@v3.29.0
go get github.com/testcontainers/testcontainers-go@v0.44.0 github.com/testcontainers/testcontainers-go/modules/postgres@v0.44.0
```

- [ ] **Step 2: Write migration and embed it**

**File:** `backend/migrations/00001_init.sql`
```sql
-- +goose Up
CREATE TABLE members (
  telegram_id BIGINT PRIMARY KEY,
  name        TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE expenses (
  id          BIGSERIAL PRIMARY KEY,
  user_id     BIGINT NOT NULL REFERENCES members(telegram_id),
  amount      BIGINT NOT NULL CHECK (amount > 0),
  category    TEXT   NOT NULL CHECK (category IN
                ('food','transport','housing','entertainment','health','shopping','other')),
  spent_at    DATE   NOT NULL,
  comment     TEXT   NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX expenses_user_spent_idx ON expenses (user_id, spent_at DESC);
CREATE INDEX expenses_spent_idx      ON expenses (spent_at DESC);

CREATE TABLE chat_messages (
  id          BIGSERIAL PRIMARY KEY,
  user_id     BIGINT NOT NULL REFERENCES members(telegram_id),
  content     JSONB  NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX chat_messages_user_idx ON chat_messages (user_id, created_at DESC);

CREATE TABLE agent_traces (
  id                BIGSERIAL PRIMARY KEY,
  user_id           BIGINT NOT NULL REFERENCES members(telegram_id),
  source            TEXT   NOT NULL CHECK (source IN ('text','voice')),
  input             TEXT   NOT NULL,
  output            TEXT,
  steps             JSONB  NOT NULL,
  model             TEXT   NOT NULL,
  prompt_tokens     INT    NOT NULL DEFAULT 0,
  completion_tokens INT    NOT NULL DEFAULT 0,
  stt_ms            INT,
  latency_ms        INT    NOT NULL,
  error             TEXT,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX agent_traces_created_idx ON agent_traces (created_at DESC);

-- +goose Down
DROP TABLE agent_traces;
DROP TABLE chat_messages;
DROP TABLE expenses;
DROP TABLE members;
```

**File:** `backend/migrations/embed.go`
```go
// Package migrations embeds the goose SQL migrations.
package migrations

import "embed"

// FS holds all *.sql migrations.
//
//go:embed *.sql
var FS embed.FS
```

- [ ] **Step 3: Write sqlc config and queries**

**File:** `backend/sqlc.yaml`
```yaml
version: "2"
sql:
  - engine: postgresql
    schema: migrations
    queries: internal/store/postgres/queries
    gen:
      go:
        package: db
        out: internal/store/postgres/db
        sql_package: pgx/v5
        emit_pointers_for_null_types: true
        overrides:
          - db_type: date
            go_type: time.Time
          - db_type: date
            nullable: true
            go_type:
              type: time.Time
              pointer: true
          - db_type: pg_catalog.timestamptz
            go_type: time.Time
          - db_type: timestamptz
            go_type: time.Time
          - db_type: jsonb
            go_type: encoding/json.RawMessage
```

**File:** `backend/internal/store/postgres/queries/members.sql`
```sql
-- name: UpsertMember :exec
INSERT INTO members (telegram_id, name) VALUES ($1, $2)
ON CONFLICT (telegram_id) DO UPDATE SET name = EXCLUDED.name;

-- name: ListMembers :many
SELECT telegram_id, name FROM members ORDER BY name, telegram_id;

-- name: MemberExists :one
SELECT EXISTS (SELECT 1 FROM members WHERE telegram_id = $1);
```

**File:** `backend/internal/store/postgres/queries/expenses.sql`
```sql
-- name: InsertExpense :one
WITH ins AS (
  INSERT INTO expenses (user_id, amount, category, spent_at, comment)
  VALUES ($1, $2, $3, $4, $5)
  RETURNING *
)
SELECT ins.id, ins.user_id, m.name AS member_name, ins.amount, ins.category, ins.spent_at,
       ins.comment, ins.created_at, ins.updated_at
FROM ins JOIN members m ON m.telegram_id = ins.user_id;

-- name: GetExpense :one
SELECT e.id, e.user_id, m.name AS member_name, e.amount, e.category, e.spent_at,
       e.comment, e.created_at, e.updated_at
FROM expenses e JOIN members m ON m.telegram_id = e.user_id
WHERE e.id = sqlc.arg(id)
  AND (sqlc.narg(owner_id)::bigint IS NULL OR e.user_id = sqlc.narg(owner_id));

-- name: ListExpenses :many
SELECT e.id, e.user_id, m.name AS member_name, e.amount, e.category, e.spent_at,
       e.comment, e.created_at, e.updated_at
FROM expenses e JOIN members m ON m.telegram_id = e.user_id
WHERE (sqlc.narg(user_id)::bigint IS NULL OR e.user_id = sqlc.narg(user_id))
  AND (sqlc.narg(date_from)::date IS NULL OR e.spent_at >= sqlc.narg(date_from))
  AND (sqlc.narg(date_to)::date   IS NULL OR e.spent_at <= sqlc.narg(date_to))
  AND (sqlc.narg(category)::text  IS NULL OR e.category = sqlc.narg(category))
  AND (sqlc.narg(text)::text      IS NULL OR e.comment ILIKE '%' || sqlc.narg(text) || '%')
ORDER BY
  CASE WHEN sqlc.arg(sort_recent)::bool THEN e.created_at END DESC,
  e.spent_at DESC, e.id DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: SummarizeExpenses :one
SELECT count(*)::bigint AS cnt, coalesce(sum(e.amount), 0)::bigint AS total
FROM expenses e
WHERE (sqlc.narg(user_id)::bigint IS NULL OR e.user_id = sqlc.narg(user_id))
  AND (sqlc.narg(date_from)::date IS NULL OR e.spent_at >= sqlc.narg(date_from))
  AND (sqlc.narg(date_to)::date   IS NULL OR e.spent_at <= sqlc.narg(date_to))
  AND (sqlc.narg(category)::text  IS NULL OR e.category = sqlc.narg(category))
  AND (sqlc.narg(text)::text      IS NULL OR e.comment ILIKE '%' || sqlc.narg(text) || '%');

-- name: UpdateExpense :one
WITH upd AS (
  UPDATE expenses SET
    user_id = $2, amount = $3, category = $4, spent_at = $5, comment = $6, updated_at = now()
  WHERE expenses.id = $1
  RETURNING *
)
SELECT upd.id, upd.user_id, m.name AS member_name, upd.amount, upd.category, upd.spent_at,
       upd.comment, upd.created_at, upd.updated_at
FROM upd JOIN members m ON m.telegram_id = upd.user_id;

-- name: DeleteExpense :execrows
DELETE FROM expenses WHERE id = $1;

-- name: AggregateExpenses :many
SELECT e.spent_at, e.user_id, m.name AS member_name, e.category,
       sum(e.amount)::bigint AS total, count(*)::int AS cnt
FROM expenses e JOIN members m ON m.telegram_id = e.user_id
WHERE e.spent_at >= sqlc.arg(date_from) AND e.spent_at <= sqlc.arg(date_to)
  AND (sqlc.narg(user_id)::bigint IS NULL OR e.user_id = sqlc.narg(user_id))
  AND (sqlc.narg(category)::text  IS NULL OR e.category = sqlc.narg(category))
GROUP BY e.spent_at, e.user_id, m.name, e.category
ORDER BY e.spent_at, e.user_id, e.category;
```

**File:** `backend/internal/store/postgres/queries/agent.sql`
```sql
-- name: InsertChatMessage :exec
INSERT INTO chat_messages (user_id, content) VALUES ($1, $2);

-- name: RecentChatMessages :many
SELECT content FROM (
  SELECT id, content FROM chat_messages
  WHERE user_id = $1 AND created_at >= sqlc.arg(since)
  ORDER BY id DESC
  LIMIT sqlc.arg(lim)
) recent ORDER BY id;

-- name: InsertTrace :exec
INSERT INTO agent_traces (user_id, source, input, output, steps, model,
  prompt_tokens, completion_tokens, stt_ms, latency_ms, error)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: ListTraces :many
SELECT t.id, t.user_id, m.name AS member_name, t.source, t.input, t.output, t.steps, t.model,
       t.prompt_tokens, t.completion_tokens, t.stt_ms, t.latency_ms, t.error, t.created_at
FROM agent_traces t JOIN members m ON m.telegram_id = t.user_id
WHERE (sqlc.narg(user_id)::bigint IS NULL OR t.user_id = sqlc.narg(user_id))
ORDER BY t.id DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountTraces :one
SELECT count(*)::bigint FROM agent_traces
WHERE (sqlc.narg(user_id)::bigint IS NULL OR user_id = sqlc.narg(user_id));
```

- [ ] **Step 4: Generate sqlc code**

Run: `make gen-sql`
Expected: files `backend/internal/store/postgres/db/{db.go,models.go,members.sql.go,expenses.sql.go,agent.sql.go}`. Generated names used below (verified against sqlc 1.31.1): `db.New`, `(*Queries).WithTx`, `db.UpsertMemberParams{TelegramID, Name}`, `db.ListMembersRow{TelegramID, Name}`, `db.InsertExpenseParams{UserID, Amount, Category, SpentAt, Comment}`, row types `InsertExpenseRow`/`GetExpenseRow`/`ListExpensesRow`/`UpdateExpenseRow` with identical fields `{ID, UserID, MemberName, Amount, Category, SpentAt, Comment, CreatedAt, UpdatedAt}`, `db.GetExpenseParams{ID, OwnerID *int64}`, `db.ListExpensesParams{UserID *int64, DateFrom, DateTo *time.Time, Category, Text *string, SortRecent bool, Off, Lim int32}`, `db.SummarizeExpensesParams{UserID, DateFrom, DateTo, Category, Text}` → `db.SummarizeExpensesRow{Cnt, Total int64}`, `db.UpdateExpenseParams{ID, UserID, Amount, Category, SpentAt, Comment}`, `DeleteExpense(ctx, id) (int64, error)`, `db.AggregateExpensesParams{DateFrom, DateTo time.Time, UserID *int64, Category *string}` → `db.AggregateExpensesRow{SpentAt, UserID, MemberName, Category, Total int64, Cnt int32}`, `db.InsertChatMessageParams{UserID, Content json.RawMessage}`, `db.RecentChatMessagesParams{UserID, Since time.Time, Lim int32}` → `[]json.RawMessage`, `db.InsertTraceParams{UserID, Source, Input, Output *string, Steps json.RawMessage, Model, PromptTokens, CompletionTokens int32, SttMs *int32, LatencyMs int32, Error *string}`, `db.ListTracesParams{UserID *int64, Off, Lim int32}` → `db.ListTracesRow{ID, UserID, MemberName, Source, Input, Output, Steps, Model, PromptTokens, CompletionTokens, SttMs, LatencyMs, Error, CreatedAt}`, `CountTraces(ctx, userID *int64) (int64, error)`.

- [ ] **Step 5: Write the test helper and failing tests**

**File:** `backend/internal/testutil/pgtest/pgtest.go`
```go
// Package pgtest provides a shared, migrated Postgres testcontainer.
package pgtest

import (
	"context"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/store/postgres"
)

var (
	once     sync.Once
	pool     *pgxpool.Pool
	startErr error
)

// Pool returns a migrated pool with all tables truncated. The container is shared
// within the test binary and removed by testcontainers' reaper afterwards.
func Pool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	if testing.Short() {
		t.Skip("integration test: requires Docker")
	}
	once.Do(start)
	if startErr != nil {
		t.Fatalf("start postgres: %v", startErr)
	}
	_, err := pool.Exec(context.Background(),
		"TRUNCATE agent_traces, chat_messages, expenses, members RESTART IDENTITY CASCADE")
	if err != nil {
		t.Fatalf("truncate: %v", err)
	}
	return pool
}

func start() {
	ctx := context.Background()
	ctr, err := tcpostgres.Run(ctx, "postgres:16",
		tcpostgres.WithDatabase("expenses"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		tcpostgres.BasicWaitStrategies(),
	)
	if err != nil {
		startErr = err
		return
	}
	dsn, err := ctr.ConnectionString(ctx, "sslmode=disable")
	if err != nil {
		startErr = err
		return
	}
	p, err := postgres.Open(ctx, dsn)
	if err != nil {
		startErr = err
		return
	}
	if err := postgres.Migrate(ctx, p); err != nil {
		startErr = err
		return
	}
	pool = p
}
```

**File:** `backend/internal/store/postgres/postgres_test.go`
```go
package postgres_test

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/expensetest"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/store/postgres"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/testutil/pgtest"
)

func TestRepositoryContract(t *testing.T) {
	expensetest.RunRepositoryContract(t, func(t *testing.T) expense.Repository {
		return postgres.New(pgtest.Pool(t))
	})
}

func newStore(t *testing.T) *postgres.Store {
	t.Helper()
	s := postgres.New(pgtest.Pool(t))
	require.NoError(t, s.UpsertMembers(context.Background(), []domain.Member{expensetest.Papa, expensetest.Mama}))
	return s
}

func TestChatHistory(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	papa := expensetest.Papa.TelegramID

	msgs := []json.RawMessage{
		json.RawMessage(`{"role":"user","content":"кофе 350"}`),
		json.RawMessage(`{"role":"assistant","content":"Записал"}`),
		json.RawMessage(`{"role":"user","content":"такси 1200"}`),
	}
	require.NoError(t, s.AppendChat(ctx, papa, msgs))
	require.NoError(t, s.AppendChat(ctx, expensetest.Mama.TelegramID, []json.RawMessage{json.RawMessage(`{"role":"user","content":"чужое"}`)}))

	got, err := s.RecentChat(ctx, papa, time.Now().Add(-time.Hour), 10)
	require.NoError(t, err)
	require.Len(t, got, 3)
	assert.JSONEq(t, string(msgs[0]), string(got[0]))
	assert.JSONEq(t, string(msgs[2]), string(got[2]))

	got, err = s.RecentChat(ctx, papa, time.Now().Add(-time.Hour), 2)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.JSONEq(t, string(msgs[1]), string(got[0]), "limit keeps the latest messages in chronological order")

	got, err = s.RecentChat(ctx, papa, time.Now().Add(time.Hour), 10)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestTraces(t *testing.T) {
	ctx := context.Background()
	s := newStore(t)
	stt := 420

	require.NoError(t, s.SaveTrace(ctx, domain.Trace{
		UserID: expensetest.Papa.TelegramID, Source: "voice", Input: "кофе 350", Output: "Записал",
		Steps: []domain.TraceStep{{Tool: "add_expense", Args: json.RawMessage(`{"amount":"350"}`), Result: json.RawMessage(`{"created":{"id":1}}`), Ms: 12}},
		Model: "m", PromptTokens: 100, CompletionTokens: 20, STTms: &stt, LatencyMs: 900,
	}))
	require.NoError(t, s.SaveTrace(ctx, domain.Trace{
		UserID: expensetest.Mama.TelegramID, Source: "text", Input: "привет", Model: "m", LatencyMs: 300, Error: "llm: timeout",
	}))

	all, total, err := s.ListTraces(ctx, nil, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 2, total)
	require.Len(t, all, 2)
	assert.Equal(t, "Мама", all[0].MemberName, "newest first")
	assert.Equal(t, "llm: timeout", all[0].Error)
	assert.Empty(t, all[0].Output)
	assert.Nil(t, all[0].STTms)
	assert.Empty(t, all[0].Steps)

	papaID := expensetest.Papa.TelegramID
	papaOnly, total, err := s.ListTraces(ctx, &papaID, 10, 0)
	require.NoError(t, err)
	assert.Equal(t, 1, total)
	require.Len(t, papaOnly, 1)
	tr := papaOnly[0]
	assert.Equal(t, "voice", tr.Source)
	assert.Equal(t, "Записал", tr.Output)
	require.NotNil(t, tr.STTms)
	assert.Equal(t, 420, *tr.STTms)
	require.Len(t, tr.Steps, 1)
	assert.Equal(t, "add_expense", tr.Steps[0].Tool)
	assert.JSONEq(t, `{"amount":"350"}`, string(tr.Steps[0].Args))
	assert.False(t, tr.CreatedAt.IsZero())
}
```

- [ ] **Step 6: Run tests to verify they fail**

Run: `cd backend && go test ./internal/store/... ./internal/testutil/...`
Expected: FAIL — `undefined: postgres.Open` / `postgres.New`.

- [ ] **Step 7: Implement the store**

**File:** `backend/internal/store/postgres/postgres.go`
```go
// Package postgres implements persistence on PostgreSQL using sqlc-generated queries.
package postgres

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/store/postgres/db"
	"github.com/TheLastManStandin/TestForSemen/backend/migrations"
)

// Store implements expense.Repository and the agent's chat/trace storage.
type Store struct {
	pool *pgxpool.Pool
	q    *db.Queries
}

// New wraps a pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool, q: db.New(pool)}
}

// Open connects and waits up to ~10s for the database to answer.
func Open(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connect postgres: %w", err)
	}
	var pingErr error
	for range 10 {
		if pingErr = pool.Ping(ctx); pingErr == nil {
			return pool, nil
		}
		select {
		case <-ctx.Done():
			pool.Close()
			return nil, ctx.Err()
		case <-time.After(time.Second):
		}
	}
	pool.Close()
	return nil, fmt.Errorf("ping postgres: %w", pingErr)
}

// Migrate applies all pending goose migrations.
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations.FS)
	if err != nil {
		return fmt.Errorf("goose provider: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	return nil
}
```

**File:** `backend/internal/store/postgres/members.go`
```go
package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/store/postgres/db"
)

func (s *Store) UpsertMembers(ctx context.Context, members []domain.Member) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		for _, m := range members {
			if err := q.UpsertMember(ctx, db.UpsertMemberParams{TelegramID: m.TelegramID, Name: m.Name}); err != nil {
				return fmt.Errorf("upsert member %d: %w", m.TelegramID, err)
			}
		}
		return nil
	})
}

func (s *Store) ListMembers(ctx context.Context) ([]domain.Member, error) {
	rows, err := s.q.ListMembers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	out := make([]domain.Member, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Member{TelegramID: r.TelegramID, Name: r.Name})
	}
	return out, nil
}

func (s *Store) MemberExists(ctx context.Context, telegramID int64) (bool, error) {
	ok, err := s.q.MemberExists(ctx, telegramID)
	if err != nil {
		return false, fmt.Errorf("member exists: %w", err)
	}
	return ok, nil
}
```

**File:** `backend/internal/store/postgres/expenses.go`
```go
package postgres

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/store/postgres/db"
)

var _ expense.Repository = (*Store)(nil)

func (s *Store) Insert(ctx context.Context, e domain.Expense) (domain.Expense, error) {
	row, err := s.q.InsertExpense(ctx, db.InsertExpenseParams{
		UserID: e.UserID, Amount: int64(e.Amount), Category: string(e.Category), SpentAt: e.SpentAt, Comment: e.Comment,
	})
	if err != nil {
		return domain.Expense{}, fmt.Errorf("insert expense: %w", err)
	}
	return fromRow(db.GetExpenseRow(row)), nil
}

func (s *Store) Get(ctx context.Context, id int64, owner *int64) (domain.Expense, error) {
	row, err := s.q.GetExpense(ctx, db.GetExpenseParams{ID: id, OwnerID: owner})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Expense{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Expense{}, fmt.Errorf("get expense: %w", err)
	}
	return fromRow(row), nil
}

func (s *Store) List(ctx context.Context, q expense.ListQuery) ([]domain.Expense, error) {
	limit := q.Limit
	if limit <= 0 || limit > math.MaxInt32 {
		limit = math.MaxInt32
	}
	rows, err := s.q.ListExpenses(ctx, db.ListExpensesParams{
		UserID: q.UserID, DateFrom: q.From, DateTo: q.To, Category: categoryPtr(q.Category), Text: likePattern(q.Text),
		SortRecent: q.SortRecent, Off: int32(q.Offset), Lim: int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("list expenses: %w", err)
	}
	out := make([]domain.Expense, 0, len(rows))
	for _, r := range rows {
		out = append(out, fromRow(db.GetExpenseRow(r)))
	}
	return out, nil
}

func (s *Store) Count(ctx context.Context, q expense.ListQuery) (int, domain.Money, error) {
	row, err := s.q.SummarizeExpenses(ctx, db.SummarizeExpensesParams{
		UserID: q.UserID, DateFrom: q.From, DateTo: q.To, Category: categoryPtr(q.Category), Text: likePattern(q.Text),
	})
	if err != nil {
		return 0, 0, fmt.Errorf("count expenses: %w", err)
	}
	return int(row.Cnt), domain.Money(row.Total), nil
}

func (s *Store) Update(ctx context.Context, e domain.Expense) (domain.Expense, error) {
	row, err := s.q.UpdateExpense(ctx, db.UpdateExpenseParams{
		ID: e.ID, UserID: e.UserID, Amount: int64(e.Amount), Category: string(e.Category), SpentAt: e.SpentAt, Comment: e.Comment,
	})
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Expense{}, domain.ErrNotFound
	}
	if err != nil {
		return domain.Expense{}, fmt.Errorf("update expense: %w", err)
	}
	return fromRow(db.GetExpenseRow(row)), nil
}

func (s *Store) Delete(ctx context.Context, id int64) error {
	n, err := s.q.DeleteExpense(ctx, id)
	if err != nil {
		return fmt.Errorf("delete expense: %w", err)
	}
	if n == 0 {
		return domain.ErrNotFound
	}
	return nil
}

func (s *Store) Aggregate(ctx context.Context, q expense.AggQuery) ([]expense.AggRow, error) {
	rows, err := s.q.AggregateExpenses(ctx, db.AggregateExpensesParams{
		DateFrom: q.From, DateTo: q.To, UserID: q.UserID, Category: categoryPtr(q.Category),
	})
	if err != nil {
		return nil, fmt.Errorf("aggregate expenses: %w", err)
	}
	out := make([]expense.AggRow, 0, len(rows))
	for _, r := range rows {
		out = append(out, expense.AggRow{
			Date: r.SpentAt, UserID: r.UserID, MemberName: r.MemberName, Category: domain.Category(r.Category),
			Total: domain.Money(r.Total), Count: int(r.Cnt),
		})
	}
	return out, nil
}

func fromRow(r db.GetExpenseRow) domain.Expense {
	return domain.Expense{
		ID: r.ID, UserID: r.UserID, MemberName: r.MemberName, Amount: domain.Money(r.Amount),
		Category: domain.Category(r.Category), SpentAt: r.SpentAt, Comment: r.Comment,
		CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt,
	}
}

func categoryPtr(c *domain.Category) *string {
	if c == nil {
		return nil
	}
	s := string(*c)
	return &s
}

var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

// likePattern escapes LIKE wildcards so user text matches literally; empty means no filter.
func likePattern(text string) *string {
	if text == "" {
		return nil
	}
	s := likeEscaper.Replace(text)
	return &s
}
```

**File:** `backend/internal/store/postgres/agent.go`
```go
package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/store/postgres/db"
)

// AppendChat stores agent conversation messages (chat-completions JSON) in order.
func (s *Store) AppendChat(ctx context.Context, userID int64, msgs []json.RawMessage) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		for _, m := range msgs {
			if err := q.InsertChatMessage(ctx, db.InsertChatMessageParams{UserID: userID, Content: m}); err != nil {
				return fmt.Errorf("insert chat message: %w", err)
			}
		}
		return nil
	})
}

// RecentChat returns up to limit latest messages newer than since, oldest first.
func (s *Store) RecentChat(ctx context.Context, userID int64, since time.Time, limit int) ([]json.RawMessage, error) {
	rows, err := s.q.RecentChatMessages(ctx, db.RecentChatMessagesParams{UserID: userID, Since: since, Lim: int32(limit)})
	if err != nil {
		return nil, fmt.Errorf("recent chat: %w", err)
	}
	return rows, nil
}

// SaveTrace stores one agent run.
func (s *Store) SaveTrace(ctx context.Context, t domain.Trace) error {
	steps := t.Steps
	if steps == nil {
		steps = []domain.TraceStep{}
	}
	stepsJSON, err := json.Marshal(steps)
	if err != nil {
		return fmt.Errorf("marshal steps: %w", err)
	}
	var stt *int32
	if t.STTms != nil {
		v := int32(*t.STTms)
		stt = &v
	}
	err = s.q.InsertTrace(ctx, db.InsertTraceParams{
		UserID: t.UserID, Source: t.Source, Input: t.Input, Output: nullIfEmpty(t.Output), Steps: stepsJSON,
		Model: t.Model, PromptTokens: int32(t.PromptTokens), CompletionTokens: int32(t.CompletionTokens),
		SttMs: stt, LatencyMs: int32(t.LatencyMs), Error: nullIfEmpty(t.Error),
	})
	if err != nil {
		return fmt.Errorf("insert trace: %w", err)
	}
	return nil
}

// ListTraces returns traces newest first and the total count.
func (s *Store) ListTraces(ctx context.Context, userID *int64, limit, offset int) ([]domain.Trace, int, error) {
	rows, err := s.q.ListTraces(ctx, db.ListTracesParams{UserID: userID, Off: int32(offset), Lim: int32(limit)})
	if err != nil {
		return nil, 0, fmt.Errorf("list traces: %w", err)
	}
	total, err := s.q.CountTraces(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("count traces: %w", err)
	}
	out := make([]domain.Trace, 0, len(rows))
	for _, r := range rows {
		var steps []domain.TraceStep
		if err := json.Unmarshal(r.Steps, &steps); err != nil {
			return nil, 0, fmt.Errorf("unmarshal steps of trace %d: %w", r.ID, err)
		}
		var stt *int
		if r.SttMs != nil {
			v := int(*r.SttMs)
			stt = &v
		}
		out = append(out, domain.Trace{
			ID: r.ID, UserID: r.UserID, MemberName: r.MemberName, Source: r.Source, Input: r.Input,
			Output: deref(r.Output), Steps: steps, Model: r.Model,
			PromptTokens: int(r.PromptTokens), CompletionTokens: int(r.CompletionTokens),
			STTms: stt, LatencyMs: int(r.LatencyMs), Error: deref(r.Error), CreatedAt: r.CreatedAt,
		})
	}
	return out, int(total), nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
```

- [ ] **Step 8: Run tests to verify they pass**

Run: `cd backend && go mod tidy && go test ./internal/store/... ./internal/expense/...`
Expected: PASS. The Postgres contract run proves memrepo and Postgres behave identically (including Cyrillic `ILIKE` and escaped wildcards). If the `ТАКСИ` assertion fails on Postgres, the container is not using a UTF-8 glibc locale — confirm the image is `postgres:16`, not `-alpine`.

- [ ] **Step 9: Verify short mode skips Docker**

Run: `cd backend && go test -short ./internal/store/...`
Expected: `ok` with tests skipped (no container started).

- [ ] **Step 10: Commit**

```bash
git add backend/go.mod backend/go.sum backend/migrations backend/sqlc.yaml backend/internal/store backend/internal/testutil
git commit -m "feat(store): add postgres repository, chat history and traces"
```

---

### Task 6: In-process event bus

**Files:**
- Create: `backend/internal/events/bus.go`
- Test: `backend/internal/events/bus_test.go`

**Interfaces:**
- Consumes: `domain.Expense`.
- Produces: `type events.Type string` with `ExpenseCreated = "expense.created"`, `ExpenseUpdated = "expense.updated"`, `ExpenseDeleted = "expense.deleted"`; `type events.Event struct{ Type Type; Expense domain.Expense }`; `type events.Bus`; `func events.NewBus() *Bus`; `func (*Bus) Publish(Event)`; `func (*Bus) Subscribe() (<-chan Event, func())`.

- [ ] **Step 1: Write the failing test**

**File:** `backend/internal/events/bus_test.go`
```go
package events_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/events"
)

func receive(t *testing.T, ch <-chan events.Event) events.Event {
	t.Helper()
	select {
	case ev := <-ch:
		return ev
	case <-time.After(time.Second):
		t.Fatal("no event received")
		return events.Event{}
	}
}

func TestPublishFansOut(t *testing.T) {
	bus := events.NewBus()
	a, unsubA := bus.Subscribe()
	defer unsubA()
	b, unsubB := bus.Subscribe()
	defer unsubB()

	bus.Publish(events.Event{Type: events.ExpenseCreated, Expense: domain.Expense{ID: 7}})

	assert.Equal(t, int64(7), receive(t, a).Expense.ID)
	assert.Equal(t, events.ExpenseCreated, receive(t, b).Type)
}

func TestUnsubscribeClosesChannel(t *testing.T) {
	bus := events.NewBus()
	ch, unsub := bus.Subscribe()
	unsub()
	unsub() // idempotent
	_, ok := <-ch
	assert.False(t, ok)
	bus.Publish(events.Event{Type: events.ExpenseDeleted}) // must not panic
}

func TestSlowSubscriberDoesNotBlockPublisher(t *testing.T) {
	bus := events.NewBus()
	_, unsub := bus.Subscribe() // never read
	defer unsub()

	done := make(chan struct{})
	go func() {
		for range 1000 {
			bus.Publish(events.Event{Type: events.ExpenseUpdated})
		}
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		require.FailNow(t, "publisher blocked by slow subscriber")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd backend && go test ./internal/events/`
Expected: FAIL — `undefined: events.NewBus`.

- [ ] **Step 3: Implement**

**File:** `backend/internal/events/bus.go`
```go
// Package events is an in-process pub/sub for expense changes (feeds SSE).
package events

import (
	"sync"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

// Type is the SSE event name.
type Type string

const (
	ExpenseCreated Type = "expense.created"
	ExpenseUpdated Type = "expense.updated"
	ExpenseDeleted Type = "expense.deleted"
)

// Event describes a change to an expense. For deletions Expense is the deleted record.
type Event struct {
	Type    Type
	Expense domain.Expense
}

const subscriberBuffer = 32

// Bus fans events out to subscribers. A subscriber whose buffer is full misses
// events instead of blocking writers; SSE clients refetch on reconnect anyway.
type Bus struct {
	mu   sync.RWMutex
	subs map[chan Event]struct{}
}

// NewBus returns an empty bus.
func NewBus() *Bus { return &Bus{subs: make(map[chan Event]struct{})} }

// Publish delivers e to every subscriber without blocking.
func (b *Bus) Publish(e Event) {
	b.mu.RLock()
	defer b.mu.RUnlock()
	for ch := range b.subs {
		select {
		case ch <- e:
		default:
		}
	}
}

// Subscribe returns a channel of events and an idempotent unsubscribe func that closes it.
func (b *Bus) Subscribe() (<-chan Event, func()) {
	ch := make(chan Event, subscriberBuffer)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()

	var once sync.Once
	return ch, func() {
		once.Do(func() {
			b.mu.Lock()
			delete(b.subs, ch)
			close(ch)
			b.mu.Unlock()
		})
	}
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test -race ./internal/events/`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/events
git commit -m "feat(events): add in-process event bus"
```

---

### Task 7: Expense service (validation, scoping, stats, summaries)

**Files:**
- Create: `backend/internal/expense/service.go`, `backend/internal/expense/stats.go`
- Test: `backend/internal/expense/service_test.go`, `backend/internal/expense/stats_test.go`

**Interfaces:**
- Consumes: `expense.Repository` & query types (Task 4), `memrepo.New` (Task 4, tests), `events.Event`/types (Task 6), domain types (Task 2).
- Produces:
  - `type expense.Publisher interface{ Publish(events.Event) }`
  - `type expense.Scope struct{ UserID *int64 }`; `func expense.All() Scope`; `func expense.Own(userID int64) Scope`
  - constants `expense.MaxAmount domain.Money = 10_000_000_000`, `expense.MaxComment = 500`, `expense.DefaultLimit = 50`, `expense.MaxLimit = 200`
  - `func expense.NewService(repo Repository, pub Publisher, loc *time.Location, now func() time.Time) *Service`
  - `(*Service)`: `Now() time.Time` (in loc); `Today() time.Time` (domain date); `SyncMembers(ctx, []domain.Member) error`; `ListMembers(ctx) ([]domain.Member, error)`; `MemberExists(ctx, int64) (bool, error)`; `Create(ctx, CreateInput) (domain.Expense, error)`; `Get(ctx, Scope, id int64) (domain.Expense, error)`; `List(ctx, Scope, ListQuery) (Page, error)`; `Update(ctx, Scope, id int64, Patch) (domain.Expense, error)`; `Delete(ctx, Scope, id int64) (domain.Expense, error)`; `Stats(ctx, StatsQuery) (Stats, error)`; `Summary(ctx, Scope, SummaryQuery) (Summary, error)`
  - `type expense.CreateInput struct{ UserID int64; Amount domain.Money; Category domain.Category; SpentAt time.Time; Comment string }` (zero `SpentAt` = today)
  - `type expense.Patch struct{ UserID *int64; Amount *domain.Money; Category *domain.Category; SpentAt *time.Time; Comment *string }`
  - `type expense.Page struct{ Items []domain.Expense; Total int; Sum domain.Money }`
  - `type expense.StatsQuery struct{ From, To time.Time; UserID *int64; Category *domain.Category }`
  - `type expense.Stats struct{ Total, PrevTotal domain.Money; Count int; ByCategory []CategoryTotal; ByMember []MemberTotal; ByDay []DayTotal }` with `CategoryTotal{Category; Total; Count}`, `MemberTotal{UserID; MemberName; Total; Count}`, `DayTotal{Date time.Time; UserID int64; Total}`
  - `type expense.GroupBy string` with `GroupNone="none"`, `GroupCategory="category"`, `GroupDay="day"`; `type expense.SummaryQuery struct{ From, To time.Time; Category *domain.Category; GroupBy GroupBy }`; `type expense.Summary struct{ From, To time.Time; Total domain.Money; Count int; Groups []SummaryGroup }`; `type expense.SummaryGroup struct{ Key, Label string; Total domain.Money; Count int }`

- [ ] **Step 1: Write the failing tests**

**File:** `backend/internal/expense/service_test.go`
```go
package expense_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/events"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/expensetest"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/memrepo"
)

var (
	papa = expensetest.Papa.TelegramID
	mama = expensetest.Mama.TelegramID
	// Thursday 2026-10-15 12:00 Moscow.
	fixedNow = time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
	ctx      = context.Background()
)

type recorder struct {
	mu     sync.Mutex
	events []events.Event
}

func (r *recorder) Publish(e events.Event) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, e)
}

func (r *recorder) types() []events.Type {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]events.Type, 0, len(r.events))
	for _, e := range r.events {
		out = append(out, e.Type)
	}
	return out
}

func newService(t *testing.T, now time.Time) (*expense.Service, *recorder) {
	t.Helper()
	repo := memrepo.New()
	require.NoError(t, repo.UpsertMembers(ctx, []domain.Member{expensetest.Papa, expensetest.Mama}))
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)
	rec := &recorder{}
	return expense.NewService(repo, rec, loc, func() time.Time { return now }), rec
}

func mustCreate(t *testing.T, svc *expense.Service, user int64, amount domain.Money, cat domain.Category, date, comment string) domain.Expense {
	t.Helper()
	e, err := svc.Create(ctx, expense.CreateInput{UserID: user, Amount: amount, Category: cat, SpentAt: expensetest.Date(date), Comment: comment})
	require.NoError(t, err)
	return e
}

func fieldOf(t *testing.T, err error) string {
	t.Helper()
	var ve *domain.ValidationError
	require.True(t, errors.As(err, &ve), "want ValidationError, got %v", err)
	return ve.Field
}

func ptr[T any](v T) *T { return &v }

func TestTodayUsesLocation(t *testing.T) {
	svc, _ := newService(t, time.Date(2026, 10, 15, 22, 30, 0, 0, time.UTC))
	assert.Equal(t, "2026-10-16", domain.FormatDate(svc.Today()))
}

func TestCreateDefaultsAndPublishes(t *testing.T) {
	svc, rec := newService(t, fixedNow)
	e, err := svc.Create(ctx, expense.CreateInput{UserID: papa, Amount: 35000, Category: domain.CategoryFood, Comment: "  кофе  "})
	require.NoError(t, err)
	assert.Equal(t, "2026-10-15", domain.FormatDate(e.SpentAt))
	assert.Equal(t, "кофе", e.Comment)
	assert.Equal(t, "Папа", e.MemberName)
	assert.Equal(t, []events.Type{events.ExpenseCreated}, rec.types())
}

func TestCreateValidation(t *testing.T) {
	svc, rec := newService(t, fixedNow)
	valid := expense.CreateInput{UserID: papa, Amount: 100, Category: domain.CategoryFood, SpentAt: expensetest.Date("2026-10-15")}
	cases := []struct {
		name, field string
		mutate      func(in *expense.CreateInput)
	}{
		{"zero amount", "amount", func(in *expense.CreateInput) { in.Amount = 0 }},
		{"huge amount", "amount", func(in *expense.CreateInput) { in.Amount = expense.MaxAmount + 1 }},
		{"unknown category", "category", func(in *expense.CreateInput) { in.Category = "pets" }},
		{"future date", "spent_at", func(in *expense.CreateInput) { in.SpentAt = expensetest.Date("2026-10-17") }},
		{"long comment", "comment", func(in *expense.CreateInput) { in.Comment = strings.Repeat("я", expense.MaxComment+1) }},
		{"stranger", "user_id", func(in *expense.CreateInput) { in.UserID = 999 }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			in := valid
			c.mutate(&in)
			_, err := svc.Create(ctx, in)
			assert.Equal(t, c.field, fieldOf(t, err))
		})
	}
	assert.Empty(t, rec.types(), "failed creates publish nothing")

	tomorrow := valid
	tomorrow.SpentAt = expensetest.Date("2026-10-16")
	_, err := svc.Create(ctx, tomorrow)
	assert.NoError(t, err, "tomorrow is tolerated (timezone slack)")
}

func TestOwnScopeIsolation(t *testing.T) {
	svc, _ := newService(t, fixedNow)
	mamas := mustCreate(t, svc, mama, 500000, domain.CategoryShopping, "2026-10-12", "кроссовки")

	_, err := svc.Get(ctx, expense.Own(papa), mamas.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = svc.Update(ctx, expense.Own(papa), mamas.ID, expense.Patch{Amount: ptr(domain.Money(1))})
	assert.ErrorIs(t, err, domain.ErrNotFound)
	_, err = svc.Delete(ctx, expense.Own(papa), mamas.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)

	page, err := svc.List(ctx, expense.Own(papa), expense.ListQuery{})
	require.NoError(t, err)
	assert.Equal(t, 0, page.Total)

	got, err := svc.Get(ctx, expense.All(), mamas.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.Money(500000), got.Amount, "mama's expense is untouched")
}

func TestListOwnScopeOverridesFilter(t *testing.T) {
	svc, _ := newService(t, fixedNow)
	mustCreate(t, svc, papa, 100, domain.CategoryFood, "2026-10-14", "")
	mustCreate(t, svc, mama, 200, domain.CategoryFood, "2026-10-14", "")

	page, err := svc.List(ctx, expense.Own(papa), expense.ListQuery{UserID: ptr(mama)})
	require.NoError(t, err)
	require.Len(t, page.Items, 1)
	assert.Equal(t, papa, page.Items[0].UserID)
	assert.Equal(t, domain.Money(100), page.Sum)
}

func TestListDefaultsAndValidation(t *testing.T) {
	svc, _ := newService(t, fixedNow)
	for range 3 {
		mustCreate(t, svc, papa, 100, domain.CategoryFood, "2026-10-14", "")
	}
	page, err := svc.List(ctx, expense.All(), expense.ListQuery{})
	require.NoError(t, err)
	assert.Len(t, page.Items, 3)
	assert.Equal(t, 3, page.Total)

	_, err = svc.List(ctx, expense.All(), expense.ListQuery{From: ptr(expensetest.Date("2026-10-15")), To: ptr(expensetest.Date("2026-10-01"))})
	assert.Equal(t, "to", fieldOf(t, err))
}

func TestUpdate(t *testing.T) {
	svc, rec := newService(t, fixedNow)
	e := mustCreate(t, svc, papa, 35000, domain.CategoryFood, "2026-10-14", "кофе")

	upd, err := svc.Update(ctx, expense.Own(papa), e.ID, expense.Patch{Amount: ptr(domain.Money(45000)), Comment: ptr("")})
	require.NoError(t, err)
	assert.Equal(t, domain.Money(45000), upd.Amount)
	assert.Empty(t, upd.Comment)
	assert.Equal(t, domain.CategoryFood, upd.Category, "unpatched fields are kept")

	_, err = svc.Update(ctx, expense.Own(papa), e.ID, expense.Patch{UserID: ptr(mama)})
	assert.Equal(t, "user_id", fieldOf(t, err), "a member cannot hand an expense to someone else")

	_, err = svc.Update(ctx, expense.Own(papa), e.ID, expense.Patch{Amount: ptr(domain.Money(0))})
	assert.Equal(t, "amount", fieldOf(t, err))

	moved, err := svc.Update(ctx, expense.All(), e.ID, expense.Patch{UserID: ptr(mama)})
	require.NoError(t, err)
	assert.Equal(t, "Мама", moved.MemberName)

	assert.Equal(t, []events.Type{events.ExpenseCreated, events.ExpenseUpdated, events.ExpenseUpdated}, rec.types())
}

func TestDelete(t *testing.T) {
	svc, rec := newService(t, fixedNow)
	e := mustCreate(t, svc, papa, 120000, domain.CategoryTransport, "2026-10-14", "такси")

	deleted, err := svc.Delete(ctx, expense.Own(papa), e.ID)
	require.NoError(t, err)
	assert.Equal(t, "такси", deleted.Comment)
	_, err = svc.Get(ctx, expense.All(), e.ID)
	assert.ErrorIs(t, err, domain.ErrNotFound)
	assert.Equal(t, []events.Type{events.ExpenseCreated, events.ExpenseDeleted}, rec.types())
}
```

**File:** `backend/internal/expense/stats_test.go`
```go
package expense_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/expensetest"
)

func statsFixture(t *testing.T) *expense.Service {
	t.Helper()
	svc, _ := newService(t, fixedNow)
	mustCreate(t, svc, papa, 100, domain.CategoryFood, "2026-10-14", "кофе")
	mustCreate(t, svc, papa, 400, domain.CategoryTransport, "2026-10-15", "такси")
	mustCreate(t, svc, mama, 200, domain.CategoryFood, "2026-10-15", "обед")
	mustCreate(t, svc, papa, 50, domain.CategoryFood, "2026-10-13", "вчерашний кофе") // previous period
	return svc
}

func TestStats(t *testing.T) {
	svc := statsFixture(t)
	st, err := svc.Stats(ctx, expense.StatsQuery{From: expensetest.Date("2026-10-14"), To: expensetest.Date("2026-10-15")})
	require.NoError(t, err)

	assert.Equal(t, domain.Money(700), st.Total)
	assert.Equal(t, 3, st.Count)
	assert.Equal(t, domain.Money(50), st.PrevTotal, "previous period is 2026-10-12..13")
	assert.Equal(t, []expense.CategoryTotal{
		{Category: domain.CategoryTransport, Total: 400, Count: 1},
		{Category: domain.CategoryFood, Total: 300, Count: 2},
	}, st.ByCategory)
	assert.Equal(t, []expense.MemberTotal{
		{UserID: papa, MemberName: "Папа", Total: 500, Count: 2},
		{UserID: mama, MemberName: "Мама", Total: 200, Count: 1},
	}, st.ByMember)
	require.Len(t, st.ByDay, 3)
	assert.Equal(t, "2026-10-14", domain.FormatDate(st.ByDay[0].Date))
	assert.Equal(t, papa, st.ByDay[1].UserID)
	assert.Equal(t, mama, st.ByDay[2].UserID)
	assert.Equal(t, domain.Money(200), st.ByDay[2].Total)
}

func TestStatsEmptyHasNonNilSlices(t *testing.T) {
	svc, _ := newService(t, fixedNow)
	st, err := svc.Stats(ctx, expense.StatsQuery{From: expensetest.Date("2026-10-01"), To: expensetest.Date("2026-10-15")})
	require.NoError(t, err)
	assert.NotNil(t, st.ByCategory)
	assert.NotNil(t, st.ByMember)
	assert.NotNil(t, st.ByDay)
}

func TestStatsRejectsInvertedRange(t *testing.T) {
	svc, _ := newService(t, fixedNow)
	_, err := svc.Stats(ctx, expense.StatsQuery{From: expensetest.Date("2026-10-15"), To: expensetest.Date("2026-10-14")})
	assert.Equal(t, "to", fieldOf(t, err))
}

func TestSummaryByCategoryIsScoped(t *testing.T) {
	svc := statsFixture(t)
	sum, err := svc.Summary(ctx, expense.Own(papa), expense.SummaryQuery{
		From: expensetest.Date("2026-10-14"), To: expensetest.Date("2026-10-15"), GroupBy: expense.GroupCategory,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.Money(500), sum.Total)
	assert.Equal(t, 2, sum.Count)
	assert.Equal(t, []expense.SummaryGroup{
		{Key: "transport", Label: "транспорт", Total: 400, Count: 1},
		{Key: "food", Label: "еда", Total: 100, Count: 1},
	}, sum.Groups)
}

func TestSummaryByDay(t *testing.T) {
	svc := statsFixture(t)
	sum, err := svc.Summary(ctx, expense.Own(papa), expense.SummaryQuery{
		From: expensetest.Date("2026-10-13"), To: expensetest.Date("2026-10-15"), GroupBy: expense.GroupDay,
	})
	require.NoError(t, err)
	require.Len(t, sum.Groups, 3)
	assert.Equal(t, "2026-10-13", sum.Groups[0].Key)
	assert.Equal(t, "14.10 (среда)", sum.Groups[1].Label)
}

func TestSummaryNoneAndValidation(t *testing.T) {
	svc := statsFixture(t)
	sum, err := svc.Summary(ctx, expense.Own(papa), expense.SummaryQuery{
		From: expensetest.Date("2026-10-01"), To: expensetest.Date("2026-10-15"), Category: ptr(domain.CategoryFood), GroupBy: expense.GroupNone,
	})
	require.NoError(t, err)
	assert.Equal(t, domain.Money(150), sum.Total)
	assert.Empty(t, sum.Groups)

	_, err = svc.Summary(ctx, expense.Own(papa), expense.SummaryQuery{
		From: expensetest.Date("2026-10-01"), To: expensetest.Date("2026-10-15"), GroupBy: "week",
	})
	assert.Equal(t, "group_by", fieldOf(t, err))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/expense/`
Expected: FAIL — `undefined: expense.NewService`.

- [ ] **Step 3: Implement the service**

**File:** `backend/internal/expense/service.go`
```go
package expense

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/events"
)

// MaxAmount is 100 million roubles.
const MaxAmount domain.Money = 10_000_000_000

// Limits.
const (
	MaxComment   = 500
	DefaultLimit = 50
	MaxLimit     = 200
)

// Publisher receives events after successful mutations.
type Publisher interface {
	Publish(events.Event)
}

type noopPublisher struct{}

func (noopPublisher) Publish(events.Event) {}

// Scope limits which expenses an operation may see or touch.
type Scope struct {
	UserID *int64 // nil: all members (web panel); set: only this member (bot)
}

// All is the panel scope.
func All() Scope { return Scope{} }

// Own is a single member's scope.
func Own(userID int64) Scope { return Scope{UserID: &userID} }

// Service is the single home of expense business rules, shared by REST and the agent.
type Service struct {
	repo Repository
	pub  Publisher
	loc  *time.Location
	now  func() time.Time
}

// NewService wires a service. pub, loc and now may be nil (no-op, UTC, time.Now).
func NewService(repo Repository, pub Publisher, loc *time.Location, now func() time.Time) *Service {
	if pub == nil {
		pub = noopPublisher{}
	}
	if loc == nil {
		loc = time.UTC
	}
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, pub: pub, loc: loc, now: now}
}

// Now returns the current time in the family's location.
func (s *Service) Now() time.Time { return s.now().In(s.loc) }

// Today returns today's domain date in the family's location.
func (s *Service) Today() time.Time { return domain.DateOf(s.Now()) }

func (s *Service) SyncMembers(ctx context.Context, members []domain.Member) error {
	return s.repo.UpsertMembers(ctx, members)
}

func (s *Service) ListMembers(ctx context.Context) ([]domain.Member, error) {
	return s.repo.ListMembers(ctx)
}

func (s *Service) MemberExists(ctx context.Context, telegramID int64) (bool, error) {
	return s.repo.MemberExists(ctx, telegramID)
}

// CreateInput is a new expense. Zero SpentAt means today.
type CreateInput struct {
	UserID   int64
	Amount   domain.Money
	Category domain.Category
	SpentAt  time.Time
	Comment  string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (domain.Expense, error) {
	e := domain.Expense{
		UserID:   in.UserID,
		Amount:   in.Amount,
		Category: in.Category,
		SpentAt:  domain.DateOf(in.SpentAt),
		Comment:  strings.TrimSpace(in.Comment),
	}
	if in.SpentAt.IsZero() {
		e.SpentAt = s.Today()
	}
	if err := s.validate(ctx, e); err != nil {
		return domain.Expense{}, err
	}
	created, err := s.repo.Insert(ctx, e)
	if err != nil {
		return domain.Expense{}, err
	}
	s.pub.Publish(events.Event{Type: events.ExpenseCreated, Expense: created})
	return created, nil
}

func (s *Service) Get(ctx context.Context, scope Scope, id int64) (domain.Expense, error) {
	return s.repo.Get(ctx, id, scope.UserID)
}

// Page is one page of a listing plus totals over the whole filter.
type Page struct {
	Items []domain.Expense
	Total int
	Sum   domain.Money
}

func (s *Service) List(ctx context.Context, scope Scope, q ListQuery) (Page, error) {
	if scope.UserID != nil {
		q.UserID = scope.UserID
	}
	if q.Limit <= 0 {
		q.Limit = DefaultLimit
	}
	q.Limit = min(q.Limit, MaxLimit)
	q.Offset = max(q.Offset, 0)
	if q.From != nil && q.To != nil && q.To.Before(*q.From) {
		return Page{}, domain.Invalid("to", "конец периода раньше начала")
	}
	items, err := s.repo.List(ctx, q)
	if err != nil {
		return Page{}, err
	}
	total, sum, err := s.repo.Count(ctx, q)
	if err != nil {
		return Page{}, err
	}
	return Page{Items: items, Total: total, Sum: sum}, nil
}

// Patch holds optional field updates. Comment "" clears the comment.
type Patch struct {
	UserID   *int64
	Amount   *domain.Money
	Category *domain.Category
	SpentAt  *time.Time
	Comment  *string
}

func (s *Service) Update(ctx context.Context, scope Scope, id int64, p Patch) (domain.Expense, error) {
	e, err := s.repo.Get(ctx, id, scope.UserID)
	if err != nil {
		return domain.Expense{}, err
	}
	if p.UserID != nil {
		if scope.UserID != nil && *p.UserID != *scope.UserID {
			return domain.Expense{}, domain.Invalid("user_id", "нельзя передать трату другому члену семьи")
		}
		e.UserID = *p.UserID
	}
	if p.Amount != nil {
		e.Amount = *p.Amount
	}
	if p.Category != nil {
		e.Category = *p.Category
	}
	if p.SpentAt != nil {
		e.SpentAt = domain.DateOf(*p.SpentAt)
	}
	if p.Comment != nil {
		e.Comment = strings.TrimSpace(*p.Comment)
	}
	if err := s.validate(ctx, e); err != nil {
		return domain.Expense{}, err
	}
	updated, err := s.repo.Update(ctx, e)
	if err != nil {
		return domain.Expense{}, err
	}
	s.pub.Publish(events.Event{Type: events.ExpenseUpdated, Expense: updated})
	return updated, nil
}

// Delete removes an expense and returns what was deleted.
func (s *Service) Delete(ctx context.Context, scope Scope, id int64) (domain.Expense, error) {
	e, err := s.repo.Get(ctx, id, scope.UserID)
	if err != nil {
		return domain.Expense{}, err
	}
	if err := s.repo.Delete(ctx, id); err != nil {
		return domain.Expense{}, err
	}
	s.pub.Publish(events.Event{Type: events.ExpenseDeleted, Expense: e})
	return e, nil
}

func (s *Service) validate(ctx context.Context, e domain.Expense) error {
	switch {
	case e.Amount <= 0:
		return domain.Invalid("amount", "сумма должна быть больше нуля")
	case e.Amount > MaxAmount:
		return domain.Invalid("amount", "сумма слишком большая")
	case !e.Category.Valid():
		return domain.Invalid("category", fmt.Sprintf("неизвестная категория %q", e.Category))
	case e.SpentAt.After(s.Today().AddDate(0, 0, 1)):
		return domain.Invalid("spent_at", "дата траты не может быть в будущем")
	case e.SpentAt.Year() < 2000:
		return domain.Invalid("spent_at", "слишком старая дата")
	case utf8.RuneCountInString(e.Comment) > MaxComment:
		return domain.Invalid("comment", fmt.Sprintf("комментарий длиннее %d символов", MaxComment))
	}
	ok, err := s.repo.MemberExists(ctx, e.UserID)
	if err != nil {
		return fmt.Errorf("check member: %w", err)
	}
	if !ok {
		return domain.Invalid("user_id", "неизвестный член семьи")
	}
	return nil
}
```

**File:** `backend/internal/expense/stats.go`
```go
package expense

import (
	"context"
	"sort"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

// StatsQuery selects data for the panel's KPI cards and charts.
type StatsQuery struct {
	From, To time.Time
	UserID   *int64
	Category *domain.Category
}

type CategoryTotal struct {
	Category domain.Category
	Total    domain.Money
	Count    int
}

type MemberTotal struct {
	UserID     int64
	MemberName string
	Total      domain.Money
	Count      int
}

type DayTotal struct {
	Date   time.Time
	UserID int64
	Total  domain.Money
}

// Stats aggregates a period and compares it to the previous period of equal length.
type Stats struct {
	Total      domain.Money
	PrevTotal  domain.Money
	Count      int
	ByCategory []CategoryTotal // total desc
	ByMember   []MemberTotal   // total desc
	ByDay      []DayTotal      // date asc, then user id asc
}

func (s *Service) Stats(ctx context.Context, q StatsQuery) (Stats, error) {
	if q.To.Before(q.From) {
		return Stats{}, domain.Invalid("to", "конец периода раньше начала")
	}
	rows, err := s.repo.Aggregate(ctx, AggQuery(q))
	if err != nil {
		return Stats{}, err
	}
	days := int(q.To.Sub(q.From).Hours()/24) + 1
	prevTo := q.From.AddDate(0, 0, -1)
	prevRows, err := s.repo.Aggregate(ctx, AggQuery{From: prevTo.AddDate(0, 0, -(days - 1)), To: prevTo, UserID: q.UserID, Category: q.Category})
	if err != nil {
		return Stats{}, err
	}

	type dayKey struct {
		date time.Time
		user int64
	}
	cats := map[domain.Category]CategoryTotal{}
	members := map[int64]MemberTotal{}
	byDay := map[dayKey]DayTotal{}
	st := Stats{}
	for _, r := range rows {
		st.Total += r.Total
		st.Count += r.Count

		c := cats[r.Category]
		c.Category, c.Total, c.Count = r.Category, c.Total+r.Total, c.Count+r.Count
		cats[r.Category] = c

		m := members[r.UserID]
		m.UserID, m.MemberName, m.Total, m.Count = r.UserID, r.MemberName, m.Total+r.Total, m.Count+r.Count
		members[r.UserID] = m

		k := dayKey{r.Date, r.UserID}
		d := byDay[k]
		d.Date, d.UserID, d.Total = r.Date, r.UserID, d.Total+r.Total
		byDay[k] = d
	}
	for _, r := range prevRows {
		st.PrevTotal += r.Total
	}

	st.ByCategory = make([]CategoryTotal, 0, len(cats))
	for _, c := range cats {
		st.ByCategory = append(st.ByCategory, c)
	}
	sort.Slice(st.ByCategory, func(i, j int) bool {
		if st.ByCategory[i].Total != st.ByCategory[j].Total {
			return st.ByCategory[i].Total > st.ByCategory[j].Total
		}
		return st.ByCategory[i].Category < st.ByCategory[j].Category
	})

	st.ByMember = make([]MemberTotal, 0, len(members))
	for _, m := range members {
		st.ByMember = append(st.ByMember, m)
	}
	sort.Slice(st.ByMember, func(i, j int) bool {
		if st.ByMember[i].Total != st.ByMember[j].Total {
			return st.ByMember[i].Total > st.ByMember[j].Total
		}
		return st.ByMember[i].UserID < st.ByMember[j].UserID
	})

	st.ByDay = make([]DayTotal, 0, len(byDay))
	for _, d := range byDay {
		st.ByDay = append(st.ByDay, d)
	}
	sort.Slice(st.ByDay, func(i, j int) bool {
		if !st.ByDay[i].Date.Equal(st.ByDay[j].Date) {
			return st.ByDay[i].Date.Before(st.ByDay[j].Date)
		}
		return st.ByDay[i].UserID < st.ByDay[j].UserID
	})
	return st, nil
}

// GroupBy selects how Summary groups totals.
type GroupBy string

const (
	GroupNone     GroupBy = "none"
	GroupCategory GroupBy = "category"
	GroupDay      GroupBy = "day"
)

// SummaryQuery is what the agent's get_summary tool asks for.
type SummaryQuery struct {
	From, To time.Time
	Category *domain.Category
	GroupBy  GroupBy
}

type SummaryGroup struct {
	Key   string
	Label string
	Total domain.Money
	Count int
}

type Summary struct {
	From, To time.Time
	Total    domain.Money
	Count    int
	Groups   []SummaryGroup // category: total desc; day: chronological; none: empty
}

func (s *Service) Summary(ctx context.Context, scope Scope, q SummaryQuery) (Summary, error) {
	switch q.GroupBy {
	case GroupNone, GroupCategory, GroupDay:
	default:
		return Summary{}, domain.Invalid("group_by", "ожидается none, category или day")
	}
	if q.To.Before(q.From) {
		return Summary{}, domain.Invalid("to", "конец периода раньше начала")
	}
	rows, err := s.repo.Aggregate(ctx, AggQuery{From: q.From, To: q.To, UserID: scope.UserID, Category: q.Category})
	if err != nil {
		return Summary{}, err
	}

	sum := Summary{From: q.From, To: q.To}
	groups := map[string]*SummaryGroup{}
	var order []string
	for _, r := range rows {
		sum.Total += r.Total
		sum.Count += r.Count

		var key, label string
		switch q.GroupBy {
		case GroupCategory:
			key, label = string(r.Category), r.Category.RU()
		case GroupDay:
			key, label = domain.FormatDate(r.Date), r.Date.Format("02.01")+" ("+domain.WeekdayRU(r.Date)+")"
		default:
			continue
		}
		g, ok := groups[key]
		if !ok {
			g = &SummaryGroup{Key: key, Label: label}
			groups[key] = g
			order = append(order, key) // rows are date-ordered, so day groups stay chronological
		}
		g.Total += r.Total
		g.Count += r.Count
	}
	for _, k := range order {
		sum.Groups = append(sum.Groups, *groups[k])
	}
	if q.GroupBy == GroupCategory {
		sort.SliceStable(sum.Groups, func(i, j int) bool { return sum.Groups[i].Total > sum.Groups[j].Total })
	}
	return sum, nil
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd backend && go test -race ./internal/expense/...`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add backend/internal/expense
git commit -m "feat(expense): add service with scoping, validation, stats and summaries"
```

---

### Task 8: OpenAPI contract and REST handlers

**Files:**
- Create: `api/openapi.yaml`, `backend/oapi-codegen.yaml`
- Generate: `backend/internal/httpapi/gen/api.gen.go` (via `make gen-api`)
- Create: `backend/internal/httpapi/server.go`, `convert.go`, `errors.go`, `middleware.go`, `handler.go`
- Test: `backend/internal/httpapi/api_test.go`

**Interfaces:**
- Consumes: `*expense.Service` and its types (Task 7), `events.Bus` (Task 6), `memrepo` + `expensetest` (tests), `domain.Trace`.
- Produces:
  - `type httpapi.TraceLister interface{ ListTraces(ctx context.Context, userID *int64, limit, offset int) ([]domain.Trace, int, error) }` (implemented by `*postgres.Store`)
  - `type httpapi.Deps struct{ Service *expense.Service; Traces TraceLister; Bus *events.Bus; Health func(context.Context) error; Log *slog.Logger; Heartbeat time.Duration }`
  - `func httpapi.NewHandler(d Deps) http.Handler` — serves `/api/*` (Task 9 adds `/api/events` and `/healthz`)
  - Generated (verified with oapi-codegen v2.8.0): `gen.StrictServerInterface`, `gen.NewStrictHandlerWithOptions`, `gen.StrictHTTPServerOptions{RequestErrorHandlerFunc, ResponseErrorHandlerFunc}`, `gen.HandlerWithOptions`, `gen.StdHTTPServerOptions{BaseURL, BaseRouter, ErrorHandlerFunc}`, models `gen.Expense{Id, UserId, MemberName, Amount, Category, SpentAt openapi_types.Date, Comment *string, CreatedAt, UpdatedAt}`, `gen.ExpenseCreate`, `gen.ExpensePatch` (all pointers), `gen.ExpenseList{Items, Total, Sum}`, `gen.Member{TelegramId, Name}`, `gen.Stats{Total, PrevTotal, Count, ByCategory []CategoryTotal, ByMember []MemberTotal, ByDay []DayTotal}`, `gen.Trace{…, Output, Error *string, SttMs *int, Steps []TraceStep}`, `gen.TraceStep{Tool, Args, Result interface{}, Ms int64}`, `gen.TraceList`, `gen.Problem{Title, Status, Detail *string, Errors *[]FieldError, Type *string}`, `gen.FieldError{Field, Message}`; responses `gen.ListMembers200JSONResponse`, `gen.ListExpenses200JSONResponse`, `gen.CreateExpense201JSONResponse`, `gen.GetExpense200JSONResponse`, `gen.UpdateExpense200JSONResponse`, `gen.DeleteExpense204Response`, `gen.GetStats200JSONResponse`, `gen.ListTraces200JSONResponse`; request objects carry `Params`, `Id`, `Body`.

- [ ] **Step 1: Write the OpenAPI contract**

**File:** `api/openapi.yaml`
```yaml
openapi: 3.0.3
info:
  title: Family Expenses API
  version: 1.0.0
  description: REST API веб-панели семейного учёта расходов.
servers:
  - url: /api
paths:
  /members:
    get:
      operationId: listMembers
      summary: Члены семьи
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema:
                type: array
                items: { $ref: "#/components/schemas/Member" }
  /expenses:
    get:
      operationId: listExpenses
      summary: Список трат с фильтрами
      parameters:
        - { name: member_id, in: query, schema: { type: integer, format: int64 } }
        - { name: from, in: query, schema: { type: string, format: date } }
        - { name: to, in: query, schema: { type: string, format: date } }
        - { name: category, in: query, schema: { $ref: "#/components/schemas/Category" } }
        - { name: limit, in: query, schema: { type: integer, minimum: 1, maximum: 200, default: 50 } }
        - { name: offset, in: query, schema: { type: integer, minimum: 0, default: 0 } }
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema: { $ref: "#/components/schemas/ExpenseList" }
        "422": { $ref: "#/components/responses/Problem" }
    post:
      operationId: createExpense
      summary: Создать трату
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: "#/components/schemas/ExpenseCreate" }
      responses:
        "201":
          description: Created
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Expense" }
        "422": { $ref: "#/components/responses/Problem" }
  /expenses/{id}:
    parameters:
      - { name: id, in: path, required: true, schema: { type: integer, format: int64 } }
    get:
      operationId: getExpense
      summary: Одна трата
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Expense" }
        "404": { $ref: "#/components/responses/Problem" }
    patch:
      operationId: updateExpense
      summary: Частично обновить трату
      requestBody:
        required: true
        content:
          application/json:
            schema: { $ref: "#/components/schemas/ExpensePatch" }
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Expense" }
        "404": { $ref: "#/components/responses/Problem" }
        "422": { $ref: "#/components/responses/Problem" }
    delete:
      operationId: deleteExpense
      summary: Удалить трату
      responses:
        "204": { description: Deleted }
        "404": { $ref: "#/components/responses/Problem" }
  /stats:
    get:
      operationId: getStats
      summary: Агрегаты для KPI и графиков
      parameters:
        - { name: from, in: query, required: true, schema: { type: string, format: date } }
        - { name: to, in: query, required: true, schema: { type: string, format: date } }
        - { name: member_id, in: query, schema: { type: integer, format: int64 } }
        - { name: category, in: query, schema: { $ref: "#/components/schemas/Category" } }
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema: { $ref: "#/components/schemas/Stats" }
        "422": { $ref: "#/components/responses/Problem" }
  /traces:
    get:
      operationId: listTraces
      summary: Журнал работы агента
      parameters:
        - { name: member_id, in: query, schema: { type: integer, format: int64 } }
        - { name: limit, in: query, schema: { type: integer, minimum: 1, maximum: 100, default: 20 } }
        - { name: offset, in: query, schema: { type: integer, minimum: 0, default: 0 } }
      responses:
        "200":
          description: OK
          content:
            application/json:
              schema: { $ref: "#/components/schemas/TraceList" }
  /events:
    get:
      operationId: streamEvents
      summary: "SSE-поток событий: expense.created, expense.updated, expense.deleted"
      responses:
        "200":
          description: Поток Server-Sent Events; data — JSON ExpenseEvent
          content:
            text/event-stream:
              schema: { $ref: "#/components/schemas/ExpenseEvent" }
components:
  responses:
    Problem:
      description: Ошибка (RFC 7807)
      content:
        application/problem+json:
          schema: { $ref: "#/components/schemas/Problem" }
  schemas:
    Category:
      type: string
      enum: [food, transport, housing, entertainment, health, shopping, other]
    Member:
      type: object
      required: [telegram_id, name]
      properties:
        telegram_id: { type: integer, format: int64 }
        name: { type: string }
    Expense:
      type: object
      required: [id, user_id, member_name, amount, category, spent_at, comment, created_at, updated_at]
      properties:
        id: { type: integer, format: int64 }
        user_id: { type: integer, format: int64 }
        member_name: { type: string }
        amount: { type: string, description: "Сумма в рублях, десятичная строка", example: "350.00" }
        category: { $ref: "#/components/schemas/Category" }
        spent_at: { type: string, format: date }
        comment: { type: string, nullable: true }
        created_at: { type: string, format: date-time }
        updated_at: { type: string, format: date-time }
    ExpenseCreate:
      type: object
      required: [user_id, amount, category, spent_at]
      properties:
        user_id: { type: integer, format: int64 }
        amount: { type: string, example: "350.00" }
        category: { $ref: "#/components/schemas/Category" }
        spent_at: { type: string, format: date }
        comment: { type: string }
    ExpensePatch:
      type: object
      properties:
        user_id: { type: integer, format: int64 }
        amount: { type: string }
        category: { $ref: "#/components/schemas/Category" }
        spent_at: { type: string, format: date }
        comment: { type: string, description: "Пустая строка очищает комментарий" }
    ExpenseList:
      type: object
      required: [items, total, sum]
      properties:
        items:
          type: array
          items: { $ref: "#/components/schemas/Expense" }
        total: { type: integer, description: Количество трат по фильтру без учёта пагинации }
        sum: { type: string, description: Сумма трат по фильтру без учёта пагинации }
    Stats:
      type: object
      required: [total, prev_total, count, by_category, by_member, by_day]
      properties:
        total: { type: string }
        prev_total: { type: string, description: Сумма за предыдущий период той же длины }
        count: { type: integer }
        by_category:
          type: array
          items: { $ref: "#/components/schemas/CategoryTotal" }
        by_member:
          type: array
          items: { $ref: "#/components/schemas/MemberTotal" }
        by_day:
          type: array
          items: { $ref: "#/components/schemas/DayTotal" }
    CategoryTotal:
      type: object
      required: [category, total, count]
      properties:
        category: { $ref: "#/components/schemas/Category" }
        total: { type: string }
        count: { type: integer }
    MemberTotal:
      type: object
      required: [user_id, member_name, total, count]
      properties:
        user_id: { type: integer, format: int64 }
        member_name: { type: string }
        total: { type: string }
        count: { type: integer }
    DayTotal:
      type: object
      required: [date, user_id, total]
      properties:
        date: { type: string, format: date }
        user_id: { type: integer, format: int64 }
        total: { type: string }
    TraceStep:
      type: object
      required: [tool, args, result, ms]
      properties:
        tool: { type: string }
        args: {}
        result: {}
        ms: { type: integer, format: int64 }
    Trace:
      type: object
      required: [id, user_id, member_name, source, input, output, steps, model, prompt_tokens, completion_tokens, stt_ms, latency_ms, error, created_at]
      properties:
        id: { type: integer, format: int64 }
        user_id: { type: integer, format: int64 }
        member_name: { type: string }
        source: { type: string, enum: [text, voice] }
        input: { type: string }
        output: { type: string, nullable: true }
        steps:
          type: array
          items: { $ref: "#/components/schemas/TraceStep" }
        model: { type: string }
        prompt_tokens: { type: integer }
        completion_tokens: { type: integer }
        stt_ms: { type: integer, nullable: true }
        latency_ms: { type: integer }
        error: { type: string, nullable: true }
        created_at: { type: string, format: date-time }
    TraceList:
      type: object
      required: [items, total]
      properties:
        items:
          type: array
          items: { $ref: "#/components/schemas/Trace" }
        total: { type: integer }
    ExpenseEvent:
      type: object
      required: [type, expense]
      properties:
        type: { type: string, enum: [expense.created, expense.updated, expense.deleted] }
        expense: { $ref: "#/components/schemas/Expense" }
    Problem:
      type: object
      required: [title, status]
      properties:
        type: { type: string }
        title: { type: string }
        status: { type: integer }
        detail: { type: string }
        errors:
          type: array
          items: { $ref: "#/components/schemas/FieldError" }
    FieldError:
      type: object
      required: [field, message]
      properties:
        field: { type: string }
        message: { type: string }
```

**File:** `backend/oapi-codegen.yaml`
```yaml
package: gen
output: internal/httpapi/gen/api.gen.go
generate:
  models: true
  std-http-server: true
  strict-server: true
output-options:
  # SSE is hand-written in sse.go; it stays in the spec for documentation and frontend types.
  exclude-operation-ids: [streamEvents]
```

- [ ] **Step 2: Install codegen tool and generate**

```bash
cd backend
go get -tool github.com/oapi-codegen/oapi-codegen/v2/cmd/oapi-codegen@v2.8.0
go get github.com/oapi-codegen/runtime@v1.7.0
mkdir -p internal/httpapi/gen
cd .. && make gen-api
```
Expected: `backend/internal/httpapi/gen/api.gen.go` exists and `cd backend && go build ./internal/httpapi/gen/` succeeds.

- [ ] **Step 3: Write the failing tests**

**File:** `backend/internal/httpapi/api_test.go`
```go
package httpapi_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/events"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/expensetest"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/memrepo"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/httpapi"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/httpapi/gen"
)

var (
	ctx      = context.Background()
	papa     = expensetest.Papa.TelegramID
	mama     = expensetest.Mama.TelegramID
	fixedNow = time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC) // 12:00 Moscow
)

type fakeTraces struct{ items []domain.Trace }

func (f fakeTraces) ListTraces(_ context.Context, _ *int64, _, _ int) ([]domain.Trace, int, error) {
	return f.items, len(f.items), nil
}

type env struct {
	srv *httptest.Server
	svc *expense.Service
	bus *events.Bus
}

func newEnv(t *testing.T, opts ...func(*httpapi.Deps)) *env {
	t.Helper()
	repo := memrepo.New()
	require.NoError(t, repo.UpsertMembers(ctx, []domain.Member{expensetest.Papa, expensetest.Mama}))
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)
	bus := events.NewBus()
	svc := expense.NewService(repo, bus, loc, func() time.Time { return fixedNow })
	deps := httpapi.Deps{
		Service: svc,
		Traces:  fakeTraces{},
		Bus:     bus,
		Health:  func(context.Context) error { return nil },
		Log:     slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	for _, o := range opts {
		o(&deps)
	}
	srv := httptest.NewServer(httpapi.NewHandler(deps))
	t.Cleanup(srv.Close)
	return &env{srv: srv, svc: svc, bus: bus}
}

func (e *env) do(t *testing.T, method, path string, body any) (*http.Response, []byte) {
	t.Helper()
	var rdr io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		rdr = bytes.NewReader(b)
	}
	req, err := http.NewRequest(method, e.srv.URL+path, rdr)
	require.NoError(t, err)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, data
}

func decode[T any](t *testing.T, data []byte) T {
	t.Helper()
	var v T
	require.NoError(t, json.Unmarshal(data, &v), string(data))
	return v
}

func (e *env) create(t *testing.T, user int64, amount, category, date, comment string) gen.Expense {
	t.Helper()
	resp, data := e.do(t, http.MethodPost, "/api/expenses", map[string]any{
		"user_id": user, "amount": amount, "category": category, "spent_at": date, "comment": comment,
	})
	require.Equal(t, http.StatusCreated, resp.StatusCode, string(data))
	return decode[gen.Expense](t, data)
}

func TestCreateAndGetExpense(t *testing.T) {
	e := newEnv(t)
	created := e.create(t, papa, "350", "food", "2026-10-14", "кофе")
	assert.Equal(t, "350.00", created.Amount)
	assert.Equal(t, "Папа", created.MemberName)
	assert.Equal(t, gen.Category("food"), created.Category)
	assert.Equal(t, "2026-10-14", created.SpentAt.String())
	require.NotNil(t, created.Comment)
	assert.Equal(t, "кофе", *created.Comment)

	resp, data := e.do(t, http.MethodGet, fmt.Sprintf("/api/expenses/%d", created.Id), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, created.Id, decode[gen.Expense](t, data).Id)
}

func TestCreateValidationProblems(t *testing.T) {
	e := newEnv(t)
	cases := map[string]map[string]any{
		"amount":   {"user_id": papa, "amount": "abc", "category": "food", "spent_at": "2026-10-14"},
		"category": {"user_id": papa, "amount": "10", "category": "pets", "spent_at": "2026-10-14"},
		"user_id":  {"user_id": 999, "amount": "10", "category": "food", "spent_at": "2026-10-14"},
		"spent_at": {"user_id": papa, "amount": "10", "category": "food"},
	}
	for field, body := range cases {
		t.Run(field, func(t *testing.T) {
			resp, data := e.do(t, http.MethodPost, "/api/expenses", body)
			require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, string(data))
			assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))
			p := decode[gen.Problem](t, data)
			require.NotNil(t, p.Errors)
			assert.Equal(t, field, (*p.Errors)[0].Field)
		})
	}
}

func TestListExpensesFilters(t *testing.T) {
	e := newEnv(t)
	e.create(t, papa, "350", "food", "2026-10-14", "кофе")
	e.create(t, papa, "1200", "transport", "2026-10-14", "такси")
	e.create(t, mama, "500", "food", "2026-10-13", "обед")

	resp, data := e.do(t, http.MethodGet, fmt.Sprintf("/api/expenses?member_id=%d&category=food&from=2026-10-01&to=2026-10-15", papa), nil)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(data))
	list := decode[gen.ExpenseList](t, data)
	assert.Equal(t, 1, list.Total)
	assert.Equal(t, "350.00", list.Sum)
	require.Len(t, list.Items, 1)
	assert.Equal(t, "Папа", list.Items[0].MemberName)

	resp, data = e.do(t, http.MethodGet, "/api/expenses?category=pets", nil)
	assert.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, string(data))
}

func TestListExpensesEmptyIsArray(t *testing.T) {
	e := newEnv(t)
	resp, data := e.do(t, http.MethodGet, "/api/expenses", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Contains(t, string(data), `"items":[]`)
}

func TestListExpensesOffsetBeyondTotal(t *testing.T) {
	e := newEnv(t)
	e.create(t, papa, "100", "food", "2026-10-14", "")
	e.create(t, papa, "200", "food", "2026-10-14", "")
	_, data := e.do(t, http.MethodGet, "/api/expenses?offset=100", nil)
	list := decode[gen.ExpenseList](t, data)
	assert.Empty(t, list.Items)
	assert.Equal(t, 2, list.Total)
	assert.Equal(t, "300.00", list.Sum)
}

func TestUpdateExpense(t *testing.T) {
	e := newEnv(t)
	created := e.create(t, papa, "350", "food", "2026-10-14", "кофе")

	resp, data := e.do(t, http.MethodPatch, fmt.Sprintf("/api/expenses/%d", created.Id), map[string]any{"amount": "450", "comment": "", "user_id": mama})
	require.Equal(t, http.StatusOK, resp.StatusCode, string(data))
	upd := decode[gen.Expense](t, data)
	assert.Equal(t, "450.00", upd.Amount)
	assert.Nil(t, upd.Comment)
	assert.Equal(t, "Мама", upd.MemberName)
	assert.Contains(t, string(data), `"comment":null`)
}

func TestDeleteExpense(t *testing.T) {
	e := newEnv(t)
	created := e.create(t, papa, "350", "food", "2026-10-14", "кофе")

	resp, _ := e.do(t, http.MethodDelete, fmt.Sprintf("/api/expenses/%d", created.Id), nil)
	assert.Equal(t, http.StatusNoContent, resp.StatusCode)

	resp, data := e.do(t, http.MethodGet, fmt.Sprintf("/api/expenses/%d", created.Id), nil)
	assert.Equal(t, http.StatusNotFound, resp.StatusCode)
	assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))
	assert.Equal(t, 404, decode[gen.Problem](t, data).Status)
}

func TestBadQueryParamIsProblem(t *testing.T) {
	e := newEnv(t)
	resp, data := e.do(t, http.MethodGet, "/api/expenses?limit=abc", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))
	assert.Equal(t, 400, decode[gen.Problem](t, data).Status)
}

func TestListMembers(t *testing.T) {
	e := newEnv(t)
	_, data := e.do(t, http.MethodGet, "/api/members", nil)
	members := decode[[]gen.Member](t, data)
	assert.Equal(t, []gen.Member{{TelegramId: mama, Name: "Мама"}, {TelegramId: papa, Name: "Папа"}}, members)
}

func TestStats(t *testing.T) {
	e := newEnv(t)
	e.create(t, papa, "100", "food", "2026-10-14", "")
	e.create(t, papa, "400", "transport", "2026-10-15", "")
	e.create(t, mama, "200", "food", "2026-10-15", "")
	e.create(t, papa, "50", "food", "2026-10-13", "")

	resp, data := e.do(t, http.MethodGet, "/api/stats?from=2026-10-14&to=2026-10-15", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(data))
	st := decode[gen.Stats](t, data)
	assert.Equal(t, "700.00", st.Total)
	assert.Equal(t, "50.00", st.PrevTotal)
	assert.Equal(t, 3, st.Count)
	require.Len(t, st.ByCategory, 2)
	assert.Equal(t, gen.Category("transport"), st.ByCategory[0].Category)
	require.Len(t, st.ByMember, 2)
	assert.Equal(t, "Папа", st.ByMember[0].MemberName)
	assert.Len(t, st.ByDay, 3)

	resp, _ = e.do(t, http.MethodGet, "/api/stats?to=2026-10-15", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode, "from is required")
}

func TestListTraces(t *testing.T) {
	stt := 300
	traces := fakeTraces{items: []domain.Trace{{
		ID: 1, UserID: papa, MemberName: "Папа", Source: "voice", Input: "кофе 350", Output: "Записал",
		Steps: []domain.TraceStep{{Tool: "add_expense", Args: json.RawMessage(`{"amount":"350"}`), Result: json.RawMessage(`{"ok":true}`), Ms: 5}},
		Model: "m", PromptTokens: 10, CompletionTokens: 2, STTms: &stt, LatencyMs: 800, CreatedAt: fixedNow,
	}}}
	e := newEnv(t, func(d *httpapi.Deps) { d.Traces = traces })

	resp, data := e.do(t, http.MethodGet, "/api/traces", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(data))
	list := decode[gen.TraceList](t, data)
	assert.Equal(t, 1, list.Total)
	require.Len(t, list.Items, 1)
	tr := list.Items[0]
	assert.Equal(t, gen.TraceSource("voice"), tr.Source)
	assert.Nil(t, tr.Error)
	require.NotNil(t, tr.SttMs)
	assert.Equal(t, 300, *tr.SttMs)
	assert.Contains(t, string(data), `"args":{"amount":"350"}`)
}
```

- [ ] **Step 4: Run tests to verify they fail**

Run: `cd backend && go test ./internal/httpapi/`
Expected: FAIL — `undefined: httpapi.NewHandler`.

- [ ] **Step 5: Implement**

**File:** `backend/internal/httpapi/server.go`
```go
// Package httpapi adapts expense.Service to the REST contract in api/openapi.yaml.
package httpapi

import (
	"context"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/httpapi/gen"
)

// TraceLister reads the agent journal (implemented by postgres.Store).
type TraceLister interface {
	ListTraces(ctx context.Context, userID *int64, limit, offset int) ([]domain.Trace, int, error)
}

// Server implements the generated strict interface. Handlers return errors;
// errorHandler turns them into RFC 7807 responses.
type Server struct {
	svc    *expense.Service
	traces TraceLister
}

var _ gen.StrictServerInterface = (*Server)(nil)

func (s *Server) ListMembers(ctx context.Context, _ gen.ListMembersRequestObject) (gen.ListMembersResponseObject, error) {
	members, err := s.svc.ListMembers(ctx)
	if err != nil {
		return nil, err
	}
	out := make(gen.ListMembers200JSONResponse, 0, len(members))
	for _, m := range members {
		out = append(out, gen.Member{TelegramId: m.TelegramID, Name: m.Name})
	}
	return out, nil
}

func (s *Server) ListExpenses(ctx context.Context, req gen.ListExpensesRequestObject) (gen.ListExpensesResponseObject, error) {
	p := req.Params
	cat, err := parseCategory(p.Category)
	if err != nil {
		return nil, err
	}
	page, err := s.svc.List(ctx, expense.All(), expense.ListQuery{
		UserID: p.MemberId, From: dateOrNil(p.From), To: dateOrNil(p.To), Category: cat,
		Limit: intOr(p.Limit, expense.DefaultLimit), Offset: intOr(p.Offset, 0),
	})
	if err != nil {
		return nil, err
	}
	items := make([]gen.Expense, 0, len(page.Items))
	for _, e := range page.Items {
		items = append(items, toAPIExpense(e))
	}
	return gen.ListExpenses200JSONResponse{Items: items, Total: page.Total, Sum: page.Sum.String()}, nil
}

func (s *Server) CreateExpense(ctx context.Context, req gen.CreateExpenseRequestObject) (gen.CreateExpenseResponseObject, error) {
	b := req.Body
	if b == nil {
		return nil, domain.Invalid("body", "пустое тело запроса")
	}
	amount, err := parseAmount(b.Amount)
	if err != nil {
		return nil, err
	}
	cat, err := domain.ParseCategory(string(b.Category))
	if err != nil {
		return nil, err
	}
	if b.SpentAt.IsZero() {
		return nil, domain.Invalid("spent_at", "дата траты обязательна")
	}
	e, err := s.svc.Create(ctx, expense.CreateInput{
		UserID: b.UserId, Amount: amount, Category: cat, SpentAt: b.SpentAt.Time, Comment: deref(b.Comment),
	})
	if err != nil {
		return nil, err
	}
	return gen.CreateExpense201JSONResponse(toAPIExpense(e)), nil
}

func (s *Server) GetExpense(ctx context.Context, req gen.GetExpenseRequestObject) (gen.GetExpenseResponseObject, error) {
	e, err := s.svc.Get(ctx, expense.All(), req.Id)
	if err != nil {
		return nil, err
	}
	return gen.GetExpense200JSONResponse(toAPIExpense(e)), nil
}

func (s *Server) UpdateExpense(ctx context.Context, req gen.UpdateExpenseRequestObject) (gen.UpdateExpenseResponseObject, error) {
	b := req.Body
	if b == nil {
		return nil, domain.Invalid("body", "пустое тело запроса")
	}
	patch := expense.Patch{UserID: b.UserId, Comment: b.Comment}
	if b.Amount != nil {
		m, err := parseAmount(*b.Amount)
		if err != nil {
			return nil, err
		}
		patch.Amount = &m
	}
	if b.Category != nil {
		c, err := domain.ParseCategory(string(*b.Category))
		if err != nil {
			return nil, err
		}
		patch.Category = &c
	}
	if b.SpentAt != nil {
		t := b.SpentAt.Time
		patch.SpentAt = &t
	}
	e, err := s.svc.Update(ctx, expense.All(), req.Id, patch)
	if err != nil {
		return nil, err
	}
	return gen.UpdateExpense200JSONResponse(toAPIExpense(e)), nil
}

func (s *Server) DeleteExpense(ctx context.Context, req gen.DeleteExpenseRequestObject) (gen.DeleteExpenseResponseObject, error) {
	if _, err := s.svc.Delete(ctx, expense.All(), req.Id); err != nil {
		return nil, err
	}
	return gen.DeleteExpense204Response{}, nil
}

func (s *Server) GetStats(ctx context.Context, req gen.GetStatsRequestObject) (gen.GetStatsResponseObject, error) {
	p := req.Params
	cat, err := parseCategory(p.Category)
	if err != nil {
		return nil, err
	}
	st, err := s.svc.Stats(ctx, expense.StatsQuery{From: p.From.Time, To: p.To.Time, UserID: p.MemberId, Category: cat})
	if err != nil {
		return nil, err
	}
	return gen.GetStats200JSONResponse(toAPIStats(st)), nil
}

func (s *Server) ListTraces(ctx context.Context, req gen.ListTracesRequestObject) (gen.ListTracesResponseObject, error) {
	p := req.Params
	limit := intOr(p.Limit, 20)
	if limit < 1 || limit > 100 {
		limit = 20
	}
	traces, total, err := s.traces.ListTraces(ctx, p.MemberId, limit, max(intOr(p.Offset, 0), 0))
	if err != nil {
		return nil, err
	}
	items := make([]gen.Trace, 0, len(traces))
	for _, t := range traces {
		items = append(items, toAPITrace(t))
	}
	return gen.ListTraces200JSONResponse{Items: items, Total: total}, nil
}
```

**File:** `backend/internal/httpapi/convert.go`
```go
package httpapi

import (
	"time"

	openapi_types "github.com/oapi-codegen/runtime/types"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/httpapi/gen"
)

func toAPIExpense(e domain.Expense) gen.Expense {
	return gen.Expense{
		Id: e.ID, UserId: e.UserID, MemberName: e.MemberName, Amount: e.Amount.String(),
		Category: gen.Category(e.Category), SpentAt: openapi_types.Date{Time: e.SpentAt},
		Comment: strPtrOrNil(e.Comment), CreatedAt: e.CreatedAt, UpdatedAt: e.UpdatedAt,
	}
}

func toAPIStats(s expense.Stats) gen.Stats {
	out := gen.Stats{
		Total: s.Total.String(), PrevTotal: s.PrevTotal.String(), Count: s.Count,
		ByCategory: make([]gen.CategoryTotal, 0, len(s.ByCategory)),
		ByMember:   make([]gen.MemberTotal, 0, len(s.ByMember)),
		ByDay:      make([]gen.DayTotal, 0, len(s.ByDay)),
	}
	for _, c := range s.ByCategory {
		out.ByCategory = append(out.ByCategory, gen.CategoryTotal{Category: gen.Category(c.Category), Total: c.Total.String(), Count: c.Count})
	}
	for _, m := range s.ByMember {
		out.ByMember = append(out.ByMember, gen.MemberTotal{UserId: m.UserID, MemberName: m.MemberName, Total: m.Total.String(), Count: m.Count})
	}
	for _, d := range s.ByDay {
		out.ByDay = append(out.ByDay, gen.DayTotal{Date: openapi_types.Date{Time: d.Date}, UserId: d.UserID, Total: d.Total.String()})
	}
	return out
}

func toAPITrace(t domain.Trace) gen.Trace {
	steps := make([]gen.TraceStep, 0, len(t.Steps))
	for _, s := range t.Steps {
		steps = append(steps, gen.TraceStep{Tool: s.Tool, Args: s.Args, Result: s.Result, Ms: s.Ms})
	}
	return gen.Trace{
		Id: t.ID, UserId: t.UserID, MemberName: t.MemberName, Source: gen.TraceSource(t.Source), Input: t.Input,
		Output: strPtrOrNil(t.Output), Steps: steps, Model: t.Model, PromptTokens: t.PromptTokens,
		CompletionTokens: t.CompletionTokens, SttMs: t.STTms, LatencyMs: t.LatencyMs,
		Error: strPtrOrNil(t.Error), CreatedAt: t.CreatedAt,
	}
}

func parseCategory(c *gen.Category) (*domain.Category, error) {
	if c == nil {
		return nil, nil
	}
	cat, err := domain.ParseCategory(string(*c))
	if err != nil {
		return nil, err
	}
	return &cat, nil
}

func parseAmount(s string) (domain.Money, error) {
	m, err := domain.ParseMoney(s)
	if err != nil {
		return 0, domain.Invalid("amount", "некорректная сумма")
	}
	return m, nil
}

func dateOrNil(d *openapi_types.Date) *time.Time {
	if d == nil {
		return nil
	}
	t := d.Time
	return &t
}

func intOr(p *int, def int) int {
	if p == nil {
		return def
	}
	return *p
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func strPtrOrNil(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
```

**File:** `backend/internal/httpapi/errors.go`
```go
package httpapi

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/httpapi/gen"
)

func writeProblem(w http.ResponseWriter, status int, title, detail string, fields []gen.FieldError) {
	p := gen.Problem{Title: title, Status: status}
	if detail != "" {
		p.Detail = &detail
	}
	if len(fields) > 0 {
		p.Errors = &fields
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}

// errorHandler maps handler errors to RFC 7807 responses.
func errorHandler(log *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		var ve *domain.ValidationError
		switch {
		case errors.As(err, &ve):
			writeProblem(w, http.StatusUnprocessableEntity, "Ошибка валидации", ve.Message,
				[]gen.FieldError{{Field: ve.Field, Message: ve.Message}})
		case errors.Is(err, domain.ErrNotFound):
			writeProblem(w, http.StatusNotFound, "Не найдено", "", nil)
		default:
			log.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "err", err)
			writeProblem(w, http.StatusInternalServerError, "Внутренняя ошибка", "", nil)
		}
	}
}

// badRequest handles malformed parameters and bodies rejected by generated code.
func badRequest(w http.ResponseWriter, _ *http.Request, err error) {
	writeProblem(w, http.StatusBadRequest, "Некорректный запрос", err.Error(), nil)
}
```

**File:** `backend/internal/httpapi/middleware.go`
```go
package httpapi

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"
)

// statusWriter records the status code. Unwrap lets http.ResponseController reach
// the underlying writer, so SSE flushing keeps working through this wrapper.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (w *statusWriter) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func newRequestID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func logRequests(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		id := newRequestID()
		w.Header().Set("X-Request-Id", id)
		sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(sw, r)
		log.InfoContext(r.Context(), "http request",
			"request_id", id, "method", r.Method, "path", r.URL.Path,
			"status", sw.status, "ms", time.Since(start).Milliseconds())
	})
}

func recoverer(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if err, ok := v.(error); ok && errors.Is(err, http.ErrAbortHandler) {
					panic(v)
				}
				log.ErrorContext(r.Context(), "panic", "value", v, "stack", string(debug.Stack()))
				writeProblem(w, http.StatusInternalServerError, "Внутренняя ошибка", "", nil)
			}
		}()
		next.ServeHTTP(w, r)
	})
}
```

**File:** `backend/internal/httpapi/handler.go`
```go
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/events"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/httpapi/gen"
)

// Deps are the handler's collaborators.
type Deps struct {
	Service   *expense.Service
	Traces    TraceLister
	Bus       *events.Bus
	Health    func(context.Context) error
	Log       *slog.Logger
	Heartbeat time.Duration // SSE keep-alive interval; default 25s
}

// NewHandler builds the full HTTP handler.
func NewHandler(d Deps) http.Handler {
	if d.Heartbeat == 0 {
		d.Heartbeat = 25 * time.Second
	}
	mux := http.NewServeMux()
	strict := gen.NewStrictHandlerWithOptions(&Server{svc: d.Service, traces: d.Traces}, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  badRequest,
		ResponseErrorHandlerFunc: errorHandler(d.Log),
	})
	gen.HandlerWithOptions(strict, gen.StdHTTPServerOptions{BaseURL: "/api", BaseRouter: mux, ErrorHandlerFunc: badRequest})
	return recoverer(d.Log, logRequests(d.Log, mux))
}
```

- [ ] **Step 6: Run tests to verify they pass**

Run: `cd backend && go mod tidy && go test ./internal/httpapi/`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add api backend/oapi-codegen.yaml backend/go.mod backend/go.sum backend/internal/httpapi
git commit -m "feat(api): add OpenAPI contract and REST handlers"
```

---

### Task 9: SSE stream and health check

**Files:**
- Create: `backend/internal/httpapi/sse.go`, `backend/internal/httpapi/health.go`
- Modify: `backend/internal/httpapi/handler.go` (full replacement below)
- Test: `backend/internal/httpapi/sse_test.go`

**Interfaces:**
- Consumes: `events.Bus.Subscribe`, `toAPIExpense`, `Deps.Heartbeat`, `Deps.Health`.
- Produces: `GET /api/events` (SSE: comment `: connected` on open, `event: <type>` + `data: {"type","expense"}` per change, `: ping` every heartbeat); `GET /healthz` → `200 {"status":"ok"}` or `503 {"status":"unavailable"}`.

- [ ] **Step 1: Write the failing tests**

**File:** `backend/internal/httpapi/sse_test.go`
```go
package httpapi_test

import (
	"bufio"
	"context"
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/httpapi"
)

func readUntil(t *testing.T, r *bufio.Reader, prefix string) string {
	t.Helper()
	for {
		line, err := r.ReadString('\n')
		require.NoError(t, err, "stream ended before %q", prefix)
		if strings.HasPrefix(line, prefix) {
			return line
		}
	}
}

func TestEventsStream(t *testing.T) {
	e := newEnv(t, func(d *httpapi.Deps) { d.Heartbeat = 50 * time.Millisecond })
	reqCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, e.srv.URL+"/api/events", nil)
	require.NoError(t, err)
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, "text/event-stream", resp.Header.Get("Content-Type"))

	r := bufio.NewReader(resp.Body)
	readUntil(t, r, ": connected")

	_, err = e.svc.Create(reqCtx, expense.CreateInput{UserID: papa, Amount: 35000, Category: domain.CategoryFood, Comment: "кофе"})
	require.NoError(t, err)

	readUntil(t, r, "event: expense.created")
	data := readUntil(t, r, "data: ")
	assert.Contains(t, data, `"type":"expense.created"`)
	assert.Contains(t, data, `"comment":"кофе"`)
	assert.Contains(t, data, `"amount":"350.00"`)

	readUntil(t, r, ": ping")
}

func TestHealthz(t *testing.T) {
	e := newEnv(t)
	resp, data := e.do(t, http.MethodGet, "/healthz", nil)
	assert.Equal(t, http.StatusOK, resp.StatusCode)
	assert.JSONEq(t, `{"status":"ok"}`, string(data))

	down := newEnv(t, func(d *httpapi.Deps) {
		d.Health = func(context.Context) error { return errors.New("db down") }
	})
	resp, data = down.do(t, http.MethodGet, "/healthz", nil)
	assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
	assert.JSONEq(t, `{"status":"unavailable"}`, string(data))
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd backend && go test ./internal/httpapi/ -run 'TestEventsStream|TestHealthz'`
Expected: FAIL — 404 / wrong content type for `/api/events` and `/healthz`.

- [ ] **Step 3: Implement**

**File:** `backend/internal/httpapi/sse.go`
```go
package httpapi

import (
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/events"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/httpapi/gen"
)

type sseEvent struct {
	Type    string      `json:"type"`
	Expense gen.Expense `json:"expense"`
}

// sseHandler streams expense changes. The http.Server must not set WriteTimeout.
func sseHandler(bus *events.Bus, heartbeat time.Duration, log *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		rc := http.NewResponseController(w)
		h := w.Header()
		h.Set("Content-Type", "text/event-stream")
		h.Set("Cache-Control", "no-cache")
		h.Set("X-Accel-Buffering", "no") // disable nginx buffering

		ch, unsubscribe := bus.Subscribe()
		defer unsubscribe()

		w.WriteHeader(http.StatusOK)
		if _, err := io.WriteString(w, ": connected\n\n"); err != nil {
			return
		}
		if err := rc.Flush(); err != nil {
			log.WarnContext(r.Context(), "sse: flush unsupported", "err", err)
			return
		}

		ticker := time.NewTicker(heartbeat)
		defer ticker.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case <-ticker.C:
				if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
					return
				}
			case ev, ok := <-ch:
				if !ok {
					return
				}
				data, err := json.Marshal(sseEvent{Type: string(ev.Type), Expense: toAPIExpense(ev.Expense)})
				if err != nil {
					log.ErrorContext(r.Context(), "sse: marshal", "err", err)
					continue
				}
				if _, err := fmt.Fprintf(w, "event: %s\ndata: %s\n\n", ev.Type, data); err != nil {
					return
				}
			}
			if err := rc.Flush(); err != nil {
				return
			}
		}
	}
}
```

**File:** `backend/internal/httpapi/health.go`
```go
package httpapi

import (
	"context"
	"net/http"
	"time"
)

func healthHandler(check func(context.Context) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		w.Header().Set("Content-Type", "application/json")
		if check != nil && check(ctx) != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"unavailable"}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}
}
```

**File:** `backend/internal/httpapi/handler.go` (replace the whole file)
```go
package httpapi

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/events"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/httpapi/gen"
)

// Deps are the handler's collaborators.
type Deps struct {
	Service   *expense.Service
	Traces    TraceLister
	Bus       *events.Bus
	Health    func(context.Context) error
	Log       *slog.Logger
	Heartbeat time.Duration // SSE keep-alive interval; default 25s
}

// NewHandler builds the full HTTP handler: REST under /api, SSE at /api/events, /healthz.
func NewHandler(d Deps) http.Handler {
	if d.Heartbeat == 0 {
		d.Heartbeat = 25 * time.Second
	}
	mux := http.NewServeMux()
	strict := gen.NewStrictHandlerWithOptions(&Server{svc: d.Service, traces: d.Traces}, nil, gen.StrictHTTPServerOptions{
		RequestErrorHandlerFunc:  badRequest,
		ResponseErrorHandlerFunc: errorHandler(d.Log),
	})
	gen.HandlerWithOptions(strict, gen.StdHTTPServerOptions{BaseURL: "/api", BaseRouter: mux, ErrorHandlerFunc: badRequest})
	mux.Handle("GET /api/events", sseHandler(d.Bus, d.Heartbeat, d.Log))
	mux.Handle("GET /healthz", healthHandler(d.Health))
	return recoverer(d.Log, logRequests(d.Log, mux))
}
```

- [ ] **Step 4: Run all backend tests**

Run: `cd backend && go vet ./... && go test -race ./...`
Expected: PASS (integration tests included; Docker must be running).

- [ ] **Step 5: Commit**

```bash
git add backend/internal/httpapi
git commit -m "feat(api): add SSE event stream and health check"
```
