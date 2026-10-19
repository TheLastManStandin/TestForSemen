package agent

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/expensetest"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/memrepo"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/llm"
)

var (
	tctx     = context.Background()
	tPapa    = expensetest.Papa.TelegramID
	tMama    = expensetest.Mama.TelegramID
	tFixedAt = time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC) // Thursday 12:00 Moscow
)

func newToolAgent(t *testing.T) (*Agent, *expense.Service) {
	t.Helper()
	repo := memrepo.New()
	require.NoError(t, repo.UpsertMembers(tctx, []domain.Member{expensetest.Papa, expensetest.Mama}))
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)
	clock := func() time.Time { return tFixedAt }
	svc := expense.NewService(repo, nil, loc, clock)
	mem := NewMemoryStore(clock)
	return New(nil, svc, mem, mem, Config{Model: "test"}, slog.New(slog.NewTextHandler(io.Discard, nil))), svc
}

func call(t *testing.T, a *Agent, user int64, name, args string) map[string]any {
	t.Helper()
	o := a.execute(tctx, user, llm.ToolCall{ID: "c", Type: "function", Function: llm.FunctionCall{Name: name, Arguments: args}})
	out, step, change := o.content, o.step, o.change
	assert.Equal(t, o.failure != "", strings.Contains(out, `"error"`), "a failure reason is set exactly for failed calls: %s", out)
	assert.Equal(t, name, step.Tool)
	assert.True(t, json.Valid(step.Args), "trace args must be valid JSON")
	assert.JSONEq(t, out, string(step.Result))
	var m map[string]any
	require.NoError(t, json.Unmarshal([]byte(out), &m), out)
	_, created := m["created"]
	_, updated := m["updated"]
	_, deleted := m["deleted"]
	assert.Equal(t, created || updated || deleted, change != nil, "a change is reported exactly for successful mutations: %s", out)
	return m
}

func errorOf(m map[string]any) string {
	s, _ := m["error"].(string)
	return s
}

func listOwn(t *testing.T, svc *expense.Service, user int64) []domain.Expense {
	t.Helper()
	page, err := svc.List(tctx, expense.Own(user), expense.ListQuery{})
	require.NoError(t, err)
	return page.Items
}

func TestToolSchemasHaveNoUserID(t *testing.T) {
	data, err := json.Marshal(toolDefs)
	require.NoError(t, err)
	assert.NotContains(t, strings.ToLower(string(data)), "user")
	assert.ElementsMatch(t, []string{"add_expense", "find_expenses", "get_summary", "update_expense", "delete_expense"}, ToolNames())
	for _, td := range toolDefs {
		assert.True(t, json.Valid(td.Function.Parameters), td.Function.Name)
	}
}

func TestAddExpense(t *testing.T) {
	a, svc := newToolAgent(t)
	res := call(t, a, tPapa, "add_expense", `{"amount":"350","category":"food","comment":"кофе"}`)
	created := res["created"].(map[string]any)
	assert.Equal(t, "350 ₽", created["amount"])
	assert.Equal(t, "food", created["category"])
	assert.Equal(t, "еда", created["category_ru"])
	assert.Equal(t, "2026-10-15", created["date"])
	assert.Equal(t, "кофе", created["comment"])

	items := listOwn(t, svc, tPapa)
	require.Len(t, items, 1)
	assert.Equal(t, domain.Money(35000), items[0].Amount)
}

func TestAddExpenseAcceptsNumericAmount(t *testing.T) {
	a, svc := newToolAgent(t)
	res := call(t, a, tPapa, "add_expense", `{"amount":1200.5,"category":"transport","comment":"такси","date":"2026-10-14"}`)
	require.Empty(t, errorOf(res))
	items := listOwn(t, svc, tPapa)
	require.Len(t, items, 1)
	assert.Equal(t, domain.Money(120050), items[0].Amount)
	assert.Equal(t, "2026-10-14", domain.FormatDate(items[0].SpentAt))
}

func TestAddExpenseInvalid(t *testing.T) {
	a, svc := newToolAgent(t)
	assert.Contains(t, errorOf(call(t, a, tPapa, "add_expense", `{"amount":"350","category":"pets","comment":"x"}`)), `"category"`)
	assert.NotEmpty(t, errorOf(call(t, a, tPapa, "add_expense", `{"amount":"много","category":"food","comment":"x"}`)))
	assert.NotEmpty(t, errorOf(call(t, a, tPapa, "add_expense", `{"amount":"350","category":"food","comment":"x","date":"14.10"}`)))
	assert.Empty(t, listOwn(t, svc, tPapa))
}

func TestFindExpenses(t *testing.T) {
	a, svc := newToolAgent(t)
	for _, in := range []expense.CreateInput{
		{UserID: tPapa, Amount: 120000, Category: domain.CategoryTransport, SpentAt: expensetest.Date("2026-10-14"), Comment: "такси"},
		{UserID: tPapa, Amount: 35000, Category: domain.CategoryFood, SpentAt: expensetest.Date("2026-10-15"), Comment: "кофе"},
		{UserID: tPapa, Amount: 90000, Category: domain.CategoryTransport, SpentAt: expensetest.Date("2026-09-30"), Comment: "такси"},
	} {
		_, err := svc.Create(tctx, in)
		require.NoError(t, err)
	}

	res := call(t, a, tPapa, "find_expenses", `{"period":"this_month","text":"такси"}`)
	assert.EqualValues(t, 1, res["count"])
	assert.Equal(t, "1 200 ₽", res["total"])
	items := res["items"].([]any)
	require.Len(t, items, 1)
	assert.Equal(t, "2026-10-14", items[0].(map[string]any)["date"])
	assert.NotNil(t, items[0].(map[string]any)["id"])

	recent := call(t, a, tPapa, "find_expenses", `{"sort":"recent","limit":1}`)
	assert.Equal(t, "такси", recent["items"].([]any)[0].(map[string]any)["comment"], "most recently created")

	assert.Contains(t, errorOf(call(t, a, tPapa, "find_expenses", `{"period":"today","from":"2026-10-01"}`)), "period")
	assert.NotEmpty(t, errorOf(call(t, a, tPapa, "find_expenses", `{"period":"fortnight"}`)))
}

func TestGetSummary(t *testing.T) {
	a, svc := newToolAgent(t)
	for _, in := range []expense.CreateInput{
		{UserID: tPapa, Amount: 35000, Category: domain.CategoryFood, SpentAt: expensetest.Date("2026-10-14"), Comment: "кофе"},
		{UserID: tPapa, Amount: 50000, Category: domain.CategoryFood, SpentAt: expensetest.Date("2026-10-15"), Comment: "обед"},
		{UserID: tPapa, Amount: 120000, Category: domain.CategoryTransport, SpentAt: expensetest.Date("2026-10-15"), Comment: "такси"},
		{UserID: tMama, Amount: 999900, Category: domain.CategoryFood, SpentAt: expensetest.Date("2026-10-15"), Comment: "чужое"},
	} {
		_, err := svc.Create(tctx, in)
		require.NoError(t, err)
	}

	food := call(t, a, tPapa, "get_summary", `{"period":"this_month","category":"food"}`)
	assert.Equal(t, "850 ₽", food["total"])
	assert.EqualValues(t, 2, food["count"])
	assert.Equal(t, "2026-10-01", food["from"])

	byCat := call(t, a, tPapa, "get_summary", `{"group_by":"category"}`)
	assert.Equal(t, "2 050 ₽", byCat["total"], "defaults to this month, own expenses only")
	groups := byCat["groups"].([]any)
	require.Len(t, groups, 2)
	assert.Equal(t, "транспорт", groups[0].(map[string]any)["label"])
}

func TestUpdateAndDeleteExpense(t *testing.T) {
	a, svc := newToolAgent(t)
	e, err := svc.Create(tctx, expense.CreateInput{UserID: tPapa, Amount: 35000, Category: domain.CategoryFood, Comment: "кофе"})
	require.NoError(t, err)

	upd := call(t, a, tPapa, "update_expense", `{"id":`+itoa(e.ID)+`,"amount":"450"}`)
	assert.Equal(t, "450 ₽", upd["updated"].(map[string]any)["amount"])

	assert.Contains(t, errorOf(call(t, a, tPapa, "update_expense", `{"id":`+itoa(e.ID)+`}`)), "ни одного поля")
	assert.NotEmpty(t, errorOf(call(t, a, tPapa, "update_expense", `{"amount":"1"}`)), "id is required")

	del := call(t, a, tPapa, "delete_expense", `{"id":`+itoa(e.ID)+`}`)
	assert.Equal(t, "кофе", del["deleted"].(map[string]any)["comment"])
	assert.Empty(t, listOwn(t, svc, tPapa))
}

func TestUpdateExpenseAcceptsStringID(t *testing.T) {
	a, svc := newToolAgent(t)
	e, err := svc.Create(tctx, expense.CreateInput{UserID: tPapa, Amount: 100, Category: domain.CategoryFood, Comment: "чай"})
	require.NoError(t, err)
	res := call(t, a, tPapa, "update_expense", `{"id":"`+itoa(e.ID)+`","category":"other"}`)
	require.Empty(t, errorOf(res))
	assert.Equal(t, "other", res["updated"].(map[string]any)["category"])
}

func TestToolsCannotTouchOtherMembers(t *testing.T) {
	a, svc := newToolAgent(t)
	mamas, err := svc.Create(tctx, expense.CreateInput{UserID: tMama, Amount: 500000, Category: domain.CategoryShopping, Comment: "кроссовки"})
	require.NoError(t, err)

	found := call(t, a, tPapa, "find_expenses", `{"text":"кроссовки"}`)
	assert.EqualValues(t, 0, found["count"])
	assert.Equal(t, "трата не найдена", errorOf(call(t, a, tPapa, "update_expense", `{"id":`+itoa(mamas.ID)+`,"amount":"1"}`)))
	assert.Equal(t, "трата не найдена", errorOf(call(t, a, tPapa, "delete_expense", `{"id":`+itoa(mamas.ID)+`}`)))

	still, err := svc.Get(tctx, expense.All(), mamas.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.Money(500000), still.Amount)
}

func TestToolErrorsForBadCalls(t *testing.T) {
	a, _ := newToolAgent(t)
	assert.Contains(t, errorOf(call(t, a, tPapa, "drop_database", `{}`)), "неизвестный инструмент")
	assert.Contains(t, errorOf(call(t, a, tPapa, "add_expense", `{not json`)), "некорректные аргументы")
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// propertyOrder returns the keys of the schema's top-level "properties" in wire order.
func propertyOrder(t *testing.T, raw json.RawMessage) []string {
	t.Helper()
	var schema struct {
		Properties json.RawMessage `json:"properties"`
	}
	require.NoError(t, json.Unmarshal(raw, &schema))
	dec := json.NewDecoder(strings.NewReader(string(schema.Properties)))
	_, err := dec.Token() // {
	require.NoError(t, err)
	var keys []string
	for dec.More() {
		tok, err := dec.Token()
		require.NoError(t, err)
		keys = append(keys, tok.(string))
		var skip json.RawMessage
		require.NoError(t, dec.Decode(&skip))
	}
	return keys
}

// Regression (update_yesterday_taxi on deepseek-v4.1-flash): with properties sorted
// alphabetically "id" came last, so the model wrote {"id":10}, closed the object and
// repeated that call until the loop gave up. Properties must reach the model in the
// declared order: identifiers first, then the fields to set.
func TestToolSchemasKeepDeclaredPropertyOrder(t *testing.T) {
	want := map[string][]string{
		"add_expense":    {"amount", "category", "date", "comment"},
		"find_expenses":  {"period", "from", "to", "category", "text", "sort", "limit"},
		"get_summary":    {"period", "from", "to", "category", "group_by"},
		"update_expense": {"id", "amount", "category", "date", "comment"},
		"delete_expense": {"id"},
	}
	data, err := json.Marshal(toolDefs) // exactly what goes on the wire
	require.NoError(t, err)
	var wire []llm.Tool
	require.NoError(t, json.Unmarshal(data, &wire))
	require.Len(t, wire, len(want))
	for _, td := range wire {
		assert.Equal(t, want[td.Function.Name], propertyOrder(t, td.Function.Parameters), td.Function.Name)
	}
}

func TestToolArgsRejectUndeclaredFields(t *testing.T) {
	a, _ := newToolAgent(t)
	for _, name := range ToolNames() {
		assert.Contains(t, errorOf(call(t, a, tPapa, name, `{"user_id":2}`)), `"user_id"`, name)
	}
}

func TestUpdateWithMisspelledFieldChangesNothing(t *testing.T) {
	a, svc := newToolAgent(t)
	e, err := svc.Create(tctx, expense.CreateInput{UserID: tPapa, Amount: 120000, Category: domain.CategoryTransport, Comment: "такси"})
	require.NoError(t, err)

	res := call(t, a, tPapa, "update_expense", `{"id":`+itoa(e.ID)+`,"amout":"1300","comment":"такси домой"}`)
	assert.Contains(t, errorOf(res), `"amout"`)
	got, err := svc.Get(tctx, expense.All(), e.ID)
	require.NoError(t, err)
	assert.Equal(t, e, got, "a partly understood call must not apply the rest")
}

func TestToolArgsRequireDeclaredFields(t *testing.T) {
	a, svc := newToolAgent(t)
	for _, args := range []string{
		`{"amount":"350","category":"food"}`,
		`{"amount":"350","category":"food","comment":null}`,
	} {
		assert.Contains(t, errorOf(call(t, a, tPapa, "add_expense", args)), `"comment"`, args)
	}
	assert.Contains(t, errorOf(call(t, a, tPapa, "delete_expense", `{}`)), `"id"`)
	assert.Empty(t, listOwn(t, svc, tPapa))

	ok := call(t, a, tPapa, "add_expense", `{"amount":"350","category":"food","comment":""}`)
	assert.Empty(t, errorOf(ok), "an empty comment is a value")
}

func TestToolArgsEnforceEnums(t *testing.T) {
	a, _ := newToolAgent(t)
	assert.Contains(t, errorOf(call(t, a, tPapa, "find_expenses", `{"sort":"newest"}`)), "sort")
	assert.Contains(t, errorOf(call(t, a, tPapa, "get_summary", `{"group_by":"week"}`)), "group_by")
	assert.Empty(t, errorOf(call(t, a, tPapa, "find_expenses", `{"sort":"","period":null}`)), "empty optional values mean «not set»")
	assert.Empty(t, errorOf(call(t, a, tPapa, "get_summary", `{"group_by":"day"}`)))
}

func TestUpdateTreatsEmptyEnumAsNotSet(t *testing.T) {
	a, svc := newToolAgent(t)
	e, err := svc.Create(tctx, expense.CreateInput{UserID: tPapa, Amount: 120000, Category: domain.CategoryTransport, Comment: "такси"})
	require.NoError(t, err)

	res := call(t, a, tPapa, "update_expense", `{"id":`+itoa(e.ID)+`,"amount":"1300","category":"","date":null}`)
	require.Empty(t, errorOf(res))
	got, err := svc.Get(tctx, expense.All(), e.ID)
	require.NoError(t, err)
	assert.Equal(t, domain.Money(130000), got.Amount)
	assert.Equal(t, domain.CategoryTransport, got.Category)
}
