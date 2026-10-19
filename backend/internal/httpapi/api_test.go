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
	"strings"
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

func TestUpdateExpensePartialWithoutVersion(t *testing.T) {
	e := newEnv(t)
	created := e.create(t, papa, "350", "food", "2026-10-14", "кофе")
	assert.Equal(t, int64(1), created.Version)
	path := fmt.Sprintf("/api/expenses/%d", created.Id)

	// Bot-style change of the amount lands first; the panel then edits only the comment.
	_, err := e.svc.Update(ctx, expense.Own(papa), created.Id, expense.Patch{Amount: new(domain.Money(45000))})
	require.NoError(t, err)

	resp, data := e.do(t, http.MethodPatch, path, map[string]any{"comment": "капучино"})
	require.Equal(t, http.StatusOK, resp.StatusCode, string(data))
	upd := decode[gen.Expense](t, data)
	assert.Equal(t, "450.00", upd.Amount, "the other change survives")
	require.NotNil(t, upd.Comment)
	assert.Equal(t, "капучино", *upd.Comment)
	assert.Equal(t, gen.Category("food"), upd.Category)
	assert.Equal(t, int64(3), upd.Version)
}

func TestUpdateExpenseStaleVersionIsConflict(t *testing.T) {
	e := newEnv(t)
	created := e.create(t, papa, "350", "food", "2026-10-14", "кофе")
	path := fmt.Sprintf("/api/expenses/%d", created.Id)

	resp, data := e.do(t, http.MethodPatch, path, map[string]any{"amount": "450", "version": created.Version})
	require.Equal(t, http.StatusOK, resp.StatusCode, string(data))
	first := decode[gen.Expense](t, data)
	assert.Equal(t, created.Version+1, first.Version)

	resp, data = e.do(t, http.MethodPatch, path, map[string]any{"comment": "чай", "version": created.Version})
	require.Equal(t, http.StatusConflict, resp.StatusCode, string(data))
	assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))
	p := decode[gen.Problem](t, data)
	assert.Equal(t, http.StatusConflict, p.Status)
	require.NotNil(t, p.Detail)
	assert.Contains(t, *p.Detail, "изменена другим пользователем")

	resp, data = e.do(t, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	got := decode[gen.Expense](t, data)
	assert.Equal(t, "450.00", got.Amount)
	require.NotNil(t, got.Comment)
	assert.Equal(t, "кофе", *got.Comment, "the stale update wrote nothing")
	assert.Equal(t, first.Version, got.Version)
}

func TestUpdateExpenseEmptyPatchIsValidationError(t *testing.T) {
	e := newEnv(t)
	created := e.create(t, papa, "350", "food", "2026-10-14", "кофе")
	path := fmt.Sprintf("/api/expenses/%d", created.Id)

	for _, body := range []map[string]any{{}, {"version": created.Version}} {
		resp, data := e.do(t, http.MethodPatch, path, body)
		require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, string(data))
		p := decode[gen.Problem](t, data)
		require.NotNil(t, p.Errors)
		assert.Equal(t, "fields", (*p.Errors)[0].Field)
		assert.Contains(t, (*p.Errors)[0].Message, "не передано ни одного поля")
	}
	resp, data := e.do(t, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, created.Version, decode[gen.Expense](t, data).Version, "nothing written")
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

func TestGetSettings(t *testing.T) {
	e := newEnv(t)
	resp, data := e.do(t, http.MethodGet, "/api/settings", nil)
	require.Equal(t, http.StatusOK, resp.StatusCode, string(data))
	assert.Equal(t, gen.Settings{Timezone: "Europe/Moscow"}, decode[gen.Settings](t, data))
}

func TestRequestBodiesFollowTheContract(t *testing.T) {
	e := newEnv(t)
	created := e.create(t, papa, "350", "food", "2026-10-14", "кофе")
	path := fmt.Sprintf("/api/expenses/%d", created.Id)

	cases := []struct {
		name, method, path string
		body               map[string]any
		field, message     string
	}{
		{"unknown create field", http.MethodPost, "/api/expenses",
			map[string]any{"user_id": papa, "amount": "10", "category": "food", "spent_at": "2026-10-14", "amout": "1"}, "amout", "неизвестное поле"},
		{"misspelled patch field", http.MethodPatch, path, map[string]any{"amout": "1300", "comment": "такси"}, "amout", "неизвестное поле"},
		{"missing required field", http.MethodPost, "/api/expenses",
			map[string]any{"user_id": papa, "category": "food", "spent_at": "2026-10-14"}, "amount", "обязательное поле"},
		{"value outside enum", http.MethodPatch, path, map[string]any{"category": "pets"}, "category", "недопустимое значение"},
		{"wrong type", http.MethodPatch, path, map[string]any{"amount": 1300}, "amount", "недопустимое значение"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			resp, data := e.do(t, c.method, c.path, c.body)
			require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode, string(data))
			assert.Equal(t, "application/problem+json", resp.Header.Get("Content-Type"))
			p := decode[gen.Problem](t, data)
			require.NotNil(t, p.Errors, string(data))
			assert.Equal(t, c.field, (*p.Errors)[0].Field)
			assert.Contains(t, (*p.Errors)[0].Message, c.message)
		})
	}

	resp, data := e.do(t, http.MethodGet, path, nil)
	require.Equal(t, http.StatusOK, resp.StatusCode)
	assert.Equal(t, created, decode[gen.Expense](t, data), "rejected requests change nothing")
}

func TestMalformedRequestsAreBadRequestInRussian(t *testing.T) {
	e := newEnv(t)
	post := func(body, contentType string) (*http.Response, gen.Problem) {
		req, err := http.NewRequest(http.MethodPost, e.srv.URL+"/api/expenses", strings.NewReader(body))
		require.NoError(t, err)
		if contentType != "" {
			req.Header.Set("Content-Type", contentType)
		}
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close()
		data, err := io.ReadAll(resp.Body)
		require.NoError(t, err)
		return resp, decode[gen.Problem](t, data)
	}
	for _, c := range []struct{ body, contentType, detail string }{
		{`{"user_id":`, "application/json", "некорректный JSON"},
		{`{"user_id":1} {"x":2}`, "application/json", "некорректный JSON"},
		{`{"user_id":1}`, "", "некорректное тело запроса"},
	} {
		resp, p := post(c.body, c.contentType)
		assert.Equal(t, http.StatusBadRequest, resp.StatusCode, c.body)
		require.NotNil(t, p.Detail, c.body)
		assert.Equal(t, c.detail, *p.Detail, c.body)
	}

	resp, data := e.do(t, http.MethodGet, "/api/expenses?limit=abc", nil)
	assert.Equal(t, http.StatusBadRequest, resp.StatusCode)
	p := decode[gen.Problem](t, data)
	require.NotNil(t, p.Detail)
	assert.Equal(t, `некорректный параметр "limit"`, *p.Detail)

	resp, p = post(`{"comment":"`+strings.Repeat("x", 2<<20)+`"}`, "application/json")
	assert.Equal(t, http.StatusRequestEntityTooLarge, resp.StatusCode)
	assert.Equal(t, 413, p.Status)
}
