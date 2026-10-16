package agent_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/agent"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/expensetest"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/memrepo"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/llm"
)

var (
	ctx      = context.Background()
	papa     = expensetest.Papa.TelegramID
	mama     = expensetest.Mama.TelegramID
	fixedNow = time.Date(2026, 10, 15, 9, 0, 0, 0, time.UTC)
)

// scriptedLLM replays canned responses and records requests.
type scriptedLLM struct {
	mu       sync.Mutex
	replies  []llm.ChatResponse
	errs     map[int]error
	requests []llm.ChatRequest
}

func (s *scriptedLLM) Chat(_ context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	req.Messages = append([]llm.Message(nil), req.Messages...)
	s.requests = append(s.requests, req)
	i := len(s.requests) - 1
	if err := s.errs[i]; err != nil {
		return llm.ChatResponse{}, err
	}
	if i >= len(s.replies) {
		return llm.ChatResponse{}, errors.New("script exhausted")
	}
	return s.replies[i], nil
}

func tc(id, name, args string) llm.ToolCall {
	return llm.ToolCall{ID: id, Type: "function", Function: llm.FunctionCall{Name: name, Arguments: args}}
}

func calls(cs ...llm.ToolCall) llm.ChatResponse {
	return llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, ToolCalls: cs}, Usage: llm.Usage{PromptTokens: 100, CompletionTokens: 10}}
}

func text(s string) llm.ChatResponse {
	return llm.ChatResponse{Message: llm.Message{Role: llm.RoleAssistant, Content: s}, Usage: llm.Usage{PromptTokens: 100, CompletionTokens: 10}}
}

type fixture struct {
	agent *agent.Agent
	svc   *expense.Service
	mem   *agent.MemoryStore
	llm   *scriptedLLM
}

func newFixture(t *testing.T, client llm.Client) *fixture {
	t.Helper()
	repo := memrepo.New()
	require.NoError(t, repo.UpsertMembers(ctx, []domain.Member{expensetest.Papa, expensetest.Mama}))
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)
	clock := func() time.Time { return fixedNow }
	svc := expense.NewService(repo, nil, loc, clock)
	mem := agent.NewMemoryStore(clock)
	if client == nil {
		client = &scriptedLLM{}
	}
	scripted, _ := client.(*scriptedLLM)
	a := agent.New(client, svc, mem, mem, agent.Config{Model: "test-model"}, slog.New(slog.NewTextHandler(io.Discard, nil)))
	return &fixture{agent: a, svc: svc, mem: mem, llm: scripted}
}

func own(t *testing.T, svc *expense.Service, user int64) []domain.Expense {
	t.Helper()
	page, err := svc.List(ctx, expense.Own(user), expense.ListQuery{})
	require.NoError(t, err)
	return page.Items
}

func TestRunAddsExpenseAndReplies(t *testing.T) {
	f := newFixture(t, &scriptedLLM{replies: []llm.ChatResponse{
		calls(tc("c1", "add_expense", `{"amount":"350","category":"food","comment":"кофе"}`)),
		text("Записал: кофе — 350 ₽, еда, сегодня"),
	}})

	res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе 350"})
	require.NoError(t, err)
	assert.Equal(t, "Записал: кофе — 350 ₽, еда, сегодня", res.Reply)
	require.Len(t, res.Steps, 1)
	assert.Equal(t, "add_expense", res.Steps[0].Tool)
	assert.Len(t, own(t, f.svc, papa), 1)

	second := f.llm.requests[1].Messages
	last := second[len(second)-1]
	assert.Equal(t, llm.RoleTool, last.Role)
	assert.Equal(t, "c1", last.ToolCallID)
	assert.Contains(t, last.Content, `"created"`)

	traces := f.mem.Traces()
	require.Len(t, traces, 1)
	tr := traces[0]
	assert.Equal(t, papa, tr.UserID)
	assert.Equal(t, "text", tr.Source)
	assert.Equal(t, "кофе 350", tr.Input)
	assert.Equal(t, res.Reply, tr.Output)
	assert.Equal(t, "test-model", tr.Model)
	assert.Equal(t, 200, tr.PromptTokens)
	assert.Equal(t, 20, tr.CompletionTokens)
	assert.Empty(t, tr.Error)
	assert.Len(t, tr.Steps, 1)
}

func TestRunSendsPromptAndTools(t *testing.T) {
	f := newFixture(t, &scriptedLLM{replies: []llm.ChatResponse{text("Привет")}})
	_, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "привет"})
	require.NoError(t, err)

	req := f.llm.requests[0]
	assert.Equal(t, "test-model", req.Model)
	assert.Equal(t, llm.RoleSystem, req.Messages[0].Role)
	assert.Contains(t, req.Messages[0].Content, "Сегодня: 2026-10-15")
	assert.Equal(t, llm.Message{Role: llm.RoleUser, Content: "привет"}, req.Messages[len(req.Messages)-1])
	assert.Len(t, req.Tools, 5)
	require.NotNil(t, req.Temperature)
}

func TestRunUsesHistoryForFollowUp(t *testing.T) {
	f := newFixture(t, &scriptedLLM{replies: []llm.ChatResponse{
		calls(tc("c1", "add_expense", `{"amount":"350","category":"food","comment":"кофе"}`)),
		text("Записал"),
		text("Исправить на 450?"),
	}})
	_, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе 350"})
	require.NoError(t, err)
	_, err = f.agent.Run(ctx, agent.Input{UserID: papa, Text: "последнее было не 350, а 450"})
	require.NoError(t, err)

	msgs := f.llm.requests[2].Messages
	roles := make([]llm.Role, 0, len(msgs))
	for _, m := range msgs {
		roles = append(roles, m.Role)
	}
	assert.Equal(t, []llm.Role{llm.RoleSystem, llm.RoleUser, llm.RoleAssistant, llm.RoleTool, llm.RoleAssistant, llm.RoleUser}, roles)
	assert.Equal(t, "c1", msgs[2].ToolCalls[0].ID, "tool calls survive the round trip through history")
}

func TestRunHistoryIsPerUser(t *testing.T) {
	f := newFixture(t, &scriptedLLM{replies: []llm.ChatResponse{text("ok"), text("ok")}})
	_, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "секрет папы"})
	require.NoError(t, err)
	_, err = f.agent.Run(ctx, agent.Input{UserID: mama, Text: "привет"})
	require.NoError(t, err)
	for _, m := range f.llm.requests[1].Messages {
		assert.NotContains(t, m.Content, "секрет папы")
	}
}

func TestRunHistoryStartsAtUserMessage(t *testing.T) {
	f := newFixture(t, &scriptedLLM{replies: []llm.ChatResponse{text("ok")}})
	orphan := []json.RawMessage{
		json.RawMessage(`{"role":"tool","content":"{}","tool_call_id":"old"}`),
		json.RawMessage(`{"role":"assistant","content":"старый ответ"}`),
		json.RawMessage(`{"role":"user","content":"кофе 350"}`),
		json.RawMessage(`{"role":"assistant","content":"Записал"}`),
	}
	require.NoError(t, f.mem.AppendChat(ctx, papa, orphan))

	_, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "ещё"})
	require.NoError(t, err)
	msgs := f.llm.requests[0].Messages
	assert.Equal(t, llm.RoleUser, msgs[1].Role)
	assert.Equal(t, "кофе 350", msgs[1].Content)
	assert.Len(t, msgs, 4)
}

func TestRunParallelToolCalls(t *testing.T) {
	f := newFixture(t, &scriptedLLM{replies: []llm.ChatResponse{
		calls(
			tc("a", "add_expense", `{"amount":"350","category":"food","comment":"кофе"}`),
			tc("b", "add_expense", `{"amount":"1200","category":"transport","comment":"такси"}`),
		),
		text("Записал обе"),
	}})
	res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе 350 и такси 1200"})
	require.NoError(t, err)
	assert.Len(t, res.Steps, 2)
	assert.Len(t, own(t, f.svc, papa), 2)
	second := f.llm.requests[1].Messages
	assert.Equal(t, "a", second[len(second)-2].ToolCallID)
	assert.Equal(t, "b", second[len(second)-1].ToolCallID)
}

func TestRunStopsAfterMaxIterations(t *testing.T) {
	replies := make([]llm.ChatResponse, 0, 6)
	for i := range 6 {
		replies = append(replies, calls(tc(fmt.Sprint(i), "find_expenses", `{}`)))
	}
	f := newFixture(t, &scriptedLLM{replies: replies})

	res, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "???"})
	require.NoError(t, err, "the user gets a polite reply, not an error")
	assert.Equal(t, agent.ReplyTooManySteps, res.Reply)
	assert.Len(t, f.llm.requests, 6)
	assert.Contains(t, f.mem.Traces()[0].Error, "too many")

	history, err := f.mem.RecentChat(ctx, papa, fixedNow.Add(-time.Hour), 100)
	require.NoError(t, err)
	var last llm.Message
	require.NoError(t, json.Unmarshal(history[len(history)-1], &last))
	assert.Equal(t, llm.RoleAssistant, last.Role, "history ends cleanly")
}

func TestRunLLMErrorIsReturnedAndTraced(t *testing.T) {
	f := newFixture(t, &scriptedLLM{errs: map[int]error{0: errors.New("boom")}})
	_, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе 350", Source: agent.SourceVoice})
	require.Error(t, err)
	tr := f.mem.Traces()[0]
	assert.Contains(t, tr.Error, "boom")
	assert.Equal(t, "voice", tr.Source)
	history, _ := f.mem.RecentChat(ctx, papa, fixedNow.Add(-time.Hour), 100)
	assert.Empty(t, history, "failed turns are not stored")
}

func TestRunCannotDeleteOtherMembersExpense(t *testing.T) {
	f := newFixture(t, nil)
	mamas, err := f.svc.Create(ctx, expense.CreateInput{UserID: mama, Amount: 500000, Category: domain.CategoryShopping, Comment: "кроссовки"})
	require.NoError(t, err)
	f.llm.replies = []llm.ChatResponse{
		calls(tc("x", "delete_expense", fmt.Sprintf(`{"id":%d}`, mamas.ID))),
		text("Не нашёл такую трату"),
	}

	_, err = f.agent.Run(ctx, agent.Input{UserID: papa, Text: "удали кроссовки мамы"})
	require.NoError(t, err)
	_, err = f.svc.Get(ctx, expense.All(), mamas.ID)
	assert.NoError(t, err, "mama's expense survives")
	toolMsg := f.llm.requests[1].Messages[len(f.llm.requests[1].Messages)-1]
	assert.Contains(t, toolMsg.Content, "трата не найдена")
}

// slowLLM counts concurrent calls.
type slowLLM struct {
	inFlight, maxInFlight atomic.Int32
}

func (s *slowLLM) Chat(context.Context, llm.ChatRequest) (llm.ChatResponse, error) {
	n := s.inFlight.Add(1)
	for {
		m := s.maxInFlight.Load()
		if n <= m || s.maxInFlight.CompareAndSwap(m, n) {
			break
		}
	}
	time.Sleep(30 * time.Millisecond)
	s.inFlight.Add(-1)
	return text("ok"), nil
}

func TestRunSerializesPerUser(t *testing.T) {
	slow := &slowLLM{}
	f := newFixture(t, slow)
	var wg sync.WaitGroup
	for range 4 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = f.agent.Run(ctx, agent.Input{UserID: papa, Text: "x"})
		}()
	}
	wg.Wait()
	assert.Equal(t, int32(1), slow.maxInFlight.Load())
}

func TestRunLLMErrorAfterToolStepClosesTurn(t *testing.T) {
	f := newFixture(t, &scriptedLLM{
		replies: []llm.ChatResponse{calls(tc("c1", "add_expense", `{"amount":"350","category":"food","comment":"кофе"}`))},
		errs:    map[int]error{1: errors.New("boom")},
	})
	_, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "кофе 350"})
	require.Error(t, err)
	assert.Len(t, own(t, f.svc, papa), 1, "the side effect happened")

	raw, err := f.mem.RecentChat(ctx, papa, fixedNow.Add(-time.Hour), 100)
	require.NoError(t, err)
	var roles []llm.Role
	var last llm.Message
	for _, r := range raw {
		var m llm.Message
		require.NoError(t, json.Unmarshal(r, &m))
		roles = append(roles, m.Role)
		last = m
	}
	assert.Equal(t, []llm.Role{llm.RoleUser, llm.RoleAssistant, llm.RoleTool, llm.RoleAssistant}, roles)
	assert.Equal(t, "Не удалось закончить ответ из-за ошибки. Выполненные действия сохранены.", last.Content)
	assert.Contains(t, f.mem.Traces()[0].Error, "boom")
}

// schemaOrderLLM mimics deepseek-v4.1-flash on «вчерашнее такси было 1300»: it finds the
// expense, then writes update_expense arguments with "id" first and afterwards only the
// properties the schema lists after "id"; on a tool error it repeats the same call.
type schemaOrderLLM struct{ args string }

func (m *schemaOrderLLM) Chat(_ context.Context, req llm.ChatRequest) (llm.ChatResponse, error) {
	last := req.Messages[len(req.Messages)-1]
	if last.Role == llm.RoleUser {
		return calls(tc("f", "find_expenses", `{"period":"yesterday","text":"такси"}`)), nil
	}
	var res struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
		Error   string          `json:"error"`
		Updated json.RawMessage `json:"updated"`
	}
	if err := json.Unmarshal([]byte(last.Content), &res); err != nil {
		return llm.ChatResponse{}, err
	}
	switch {
	case res.Updated != nil:
		return text("Исправил: такси — 1 300 ₽"), nil
	case res.Error != "":
		return calls(tc("u", "update_expense", m.args)), nil
	}
	values := map[string]any{"amount": "1300"}
	args := fmt.Sprintf(`{"id":%d`, res.Items[0].ID)
	afterID := false
	for _, name := range updateSchemaOrder(req.Tools) {
		if name == "id" {
			afterID = true
			continue
		}
		if v, ok := values[name]; afterID && ok {
			b, _ := json.Marshal(v)
			args += fmt.Sprintf(`,%q:%s`, name, b)
		}
	}
	m.args = args + "}"
	return calls(tc("u", "update_expense", m.args)), nil
}

// updateSchemaOrder returns update_expense property names in the order sent on the wire.
func updateSchemaOrder(tools []llm.Tool) []string {
	data, _ := json.Marshal(tools)
	var wire []llm.Tool
	_ = json.Unmarshal(data, &wire)
	for _, t := range wire {
		if t.Function.Name != "update_expense" {
			continue
		}
		var schema struct {
			Properties json.RawMessage `json:"properties"`
		}
		_ = json.Unmarshal(t.Function.Parameters, &schema)
		dec := json.NewDecoder(bytes.NewReader(schema.Properties))
		_, _ = dec.Token()
		var names []string
		for dec.More() {
			tok, _ := dec.Token()
			names = append(names, tok.(string))
			var skip json.RawMessage
			_ = dec.Decode(&skip)
		}
		return names
	}
	return nil
}

func TestRunUpdatesAmountWhenModelFillsArgsInSchemaOrder(t *testing.T) {
	m := &schemaOrderLLM{}
	f := newFixture(t, m)
	yesterday := domain.DateOf(fixedNow).AddDate(0, 0, -1)
	taxi, err := f.svc.Create(ctx, expense.CreateInput{UserID: papa, Amount: 120_000, Category: domain.CategoryTransport, SpentAt: yesterday, Comment: "такси"})
	require.NoError(t, err)

	out, err := f.agent.Run(ctx, agent.Input{UserID: papa, Text: "вчерашнее такси было 1300"})
	require.NoError(t, err)

	assert.Equal(t, fmt.Sprintf(`{"id":%d,"amount":"1300"}`, taxi.ID), m.args)
	assert.Equal(t, "Исправил: такси — 1 300 ₽", out.Reply)
	items := own(t, f.svc, papa)
	require.Len(t, items, 1)
	assert.Equal(t, domain.Money(130_000), items[0].Amount)
}
