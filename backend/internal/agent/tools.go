package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/llm"
)

var errUnknownTool = errors.New("неизвестный инструмент")

// toolDefs are the schemas the model sees. There is deliberately no user_id anywhere:
// every tool acts on expense.Own(userID) where userID comes from the Telegram update.
var toolDefs = buildToolDefs()

// ToolNames lists the agent's tools (used by tests and evals).
func ToolNames() []string {
	names := make([]string, 0, len(toolDefs))
	for _, t := range toolDefs {
		names = append(names, t.Function.Name)
	}
	return names
}

func buildToolDefs() []llm.Tool {
	categories := make([]string, 0, len(domain.Categories))
	for _, c := range domain.Categories {
		categories = append(categories, string(c))
	}
	periods := make([]string, 0, len(domain.Periods))
	for _, p := range domain.Periods {
		periods = append(periods, string(p))
	}
	str := func(desc string) map[string]any { return map[string]any{"type": "string", "description": desc} }
	category := map[string]any{"type": "string", "enum": categories, "description": "Код категории"}
	rangeProps := func(extra ...prop) props {
		return append(props{
			{"period", map[string]any{"type": "string", "enum": periods, "description": "Относительный период; предпочтительнее from/to"}},
			{"from", str("Начало периода YYYY-MM-DD, только если period не подходит")},
			{"to", str("Конец периода YYYY-MM-DD, только если period не подходит")},
		}, extra...)
	}
	object := func(properties props, required ...string) map[string]any {
		o := map[string]any{"type": "object", "properties": properties, "additionalProperties": false}
		if len(required) > 0 {
			o["required"] = required
		}
		return o
	}
	def := func(name, desc string, params map[string]any) llm.Tool {
		raw, err := json.Marshal(params)
		if err != nil {
			panic(err)
		}
		return llm.Tool{Type: "function", Function: llm.FunctionDef{Name: name, Description: desc, Parameters: raw}}
	}

	return []llm.Tool{
		def("add_expense", "Записать новую трату пользователя.", object(props{
			{"amount", str("Сумма в рублях цифрами, например \"350\" или \"1200.50\"")},
			{"category", category},
			{"date", str("Дата траты YYYY-MM-DD; если не указана — сегодня")},
			{"comment", str("Что купили, 1–4 слова из сообщения: \"кофе\", \"такси до работы\"")},
		}, "amount", "category", "comment")),
		def("find_expenses", "Найти траты пользователя с их id: для показа списка и перед изменением или удалением.", object(rangeProps(
			prop{"category", category},
			prop{"text", str("Подстрока комментария, например \"такси\"")},
			prop{"sort", map[string]any{"type": "string", "enum": []string{"date", "recent"}, "description": "date — по дате траты, recent — последние добавленные"}},
			prop{"limit", map[string]any{"type": "integer", "minimum": 1, "maximum": 50, "description": "Сколько трат вернуть, по умолчанию 10"}},
		))),
		def("get_summary", "Посчитать сумму трат пользователя за период; без периода — текущий месяц.", object(rangeProps(
			prop{"category", category},
			prop{"group_by", map[string]any{"type": "string", "enum": []string{"none", "category", "day"}, "description": "Группировка итогов"}},
		))),
		def("update_expense", "Изменить трату пользователя по id. Передавай только изменяемые поля.", object(props{
			{"id", map[string]any{"type": "integer", "description": "id траты из find_expenses"}},
			{"amount", str("Новая сумма в рублях")},
			{"category", category},
			{"date", str("Новая дата YYYY-MM-DD")},
			{"comment", str("Новый комментарий")},
		}, "id")),
		def("delete_expense", "Удалить трату пользователя по id.", object(props{
			{"id", map[string]any{"type": "integer", "description": "id траты из find_expenses"}},
		}, "id")),
	}
}

// prop is one JSON Schema property.
type prop struct {
	name   string
	schema any
}

// props marshals as a JSON object that keeps the declared order. Models fill arguments
// in the order they see the properties; a map would sort them, putting "id" after the
// fields to change, and some models then emit {"id":N} alone.
type props []prop

func (ps props) MarshalJSON() ([]byte, error) {
	var b bytes.Buffer
	b.WriteByte('{')
	for i, p := range ps {
		if i > 0 {
			b.WriteByte(',')
		}
		k, err := json.Marshal(p.name)
		if err != nil {
			return nil, err
		}
		v, err := json.Marshal(p.schema)
		if err != nil {
			return nil, err
		}
		b.Write(k)
		b.WriteByte(':')
		b.Write(v)
	}
	b.WriteByte('}')
	return b.Bytes(), nil
}

// execute runs one tool call and returns the tool message content, a trace step and,
// for a successful add/update/delete, the change it made.
func (a *Agent) execute(ctx context.Context, userID int64, call llm.ToolCall) (string, domain.TraceStep, *Change) {
	start := time.Now()
	out, err := a.dispatch(ctx, userID, call.Function.Name, []byte(call.Function.Arguments))
	result := out
	var change *Change
	if cr, ok := out.(changeResult); ok && err == nil {
		change = &cr.change
	}
	if err != nil {
		if isInternal(err) {
			a.log.ErrorContext(ctx, "agent tool failed", "tool", call.Function.Name, "user_id", userID, "err", err)
		}
		result = map[string]string{"error": toolError(err)}
	}
	data, mErr := json.Marshal(result)
	if mErr != nil {
		data = []byte(`{"error":"внутренняя ошибка"}`)
	}
	args := json.RawMessage(call.Function.Arguments)
	if !json.Valid(args) {
		args, _ = json.Marshal(call.Function.Arguments)
	}
	return string(data), domain.TraceStep{Tool: call.Function.Name, Args: args, Result: data, Ms: time.Since(start).Milliseconds()}, change
}

func (a *Agent) dispatch(ctx context.Context, userID int64, name string, raw []byte) (any, error) {
	switch name {
	case "add_expense":
		return a.addExpense(ctx, userID, raw)
	case "find_expenses":
		return a.findExpenses(ctx, userID, raw)
	case "get_summary":
		return a.getSummary(ctx, userID, raw)
	case "update_expense":
		return a.updateExpense(ctx, userID, raw)
	case "delete_expense":
		return a.deleteExpense(ctx, userID, raw)
	}
	return nil, fmt.Errorf("%w %q", errUnknownTool, name)
}

func (a *Agent) addExpense(ctx context.Context, userID int64, raw []byte) (any, error) {
	var args struct {
		Amount   flexString `json:"amount"`
		Category string     `json:"category"`
		Date     string     `json:"date"`
		Comment  string     `json:"comment"`
	}
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	amount, err := parseAmount(string(args.Amount))
	if err != nil {
		return nil, err
	}
	cat, err := domain.ParseCategory(args.Category)
	if err != nil {
		return nil, err
	}
	var date time.Time
	if strings.TrimSpace(args.Date) != "" {
		if date, err = domain.ParseDate(args.Date); err != nil {
			return nil, err
		}
	}
	e, err := a.svc.Create(ctx, expense.CreateInput{UserID: userID, Amount: amount, Category: cat, SpentAt: date, Comment: args.Comment})
	if err != nil {
		return nil, err
	}
	return changeResult{payload: map[string]any{"created": view(e)}, change: Change{Kind: ChangeAdd, Expense: e}}, nil
}

func (a *Agent) findExpenses(ctx context.Context, userID int64, raw []byte) (any, error) {
	var args struct {
		rangeArgs
		Category string  `json:"category"`
		Text     string  `json:"text"`
		Sort     string  `json:"sort"`
		Limit    flexInt `json:"limit"`
	}
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	from, to, err := a.resolveRange(args.rangeArgs, false)
	if err != nil {
		return nil, err
	}
	cat, err := optionalCategory(args.Category)
	if err != nil {
		return nil, err
	}
	limit := int(args.Limit)
	if limit <= 0 {
		limit = 10
	}
	page, err := a.svc.List(ctx, expense.Own(userID), expense.ListQuery{
		From: from, To: to, Category: cat, Text: strings.TrimSpace(args.Text),
		SortRecent: args.Sort == "recent", Limit: min(limit, 50),
	})
	if err != nil {
		return nil, err
	}
	items := make([]expenseView, 0, len(page.Items))
	for _, e := range page.Items {
		items = append(items, view(e))
	}
	return map[string]any{"items": items, "count": page.Total, "shown": len(items), "total": page.Sum.Human()}, nil
}

func (a *Agent) getSummary(ctx context.Context, userID int64, raw []byte) (any, error) {
	var args struct {
		rangeArgs
		Category string `json:"category"`
		GroupBy  string `json:"group_by"`
	}
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	from, to, err := a.resolveRange(args.rangeArgs, true)
	if err != nil {
		return nil, err
	}
	cat, err := optionalCategory(args.Category)
	if err != nil {
		return nil, err
	}
	groupBy := expense.GroupBy(args.GroupBy)
	if groupBy == "" {
		groupBy = expense.GroupNone
	}
	sum, err := a.svc.Summary(ctx, expense.Own(userID), expense.SummaryQuery{From: *from, To: *to, Category: cat, GroupBy: groupBy})
	if err != nil {
		return nil, err
	}
	groups := make([]map[string]any, 0, len(sum.Groups))
	for _, g := range sum.Groups {
		groups = append(groups, map[string]any{"key": g.Key, "label": g.Label, "total": g.Total.Human(), "count": g.Count})
	}
	return map[string]any{
		"from": domain.FormatDate(sum.From), "to": domain.FormatDate(sum.To),
		"total": sum.Total.Human(), "count": sum.Count, "groups": groups,
	}, nil
}

func (a *Agent) updateExpense(ctx context.Context, userID int64, raw []byte) (any, error) {
	var args struct {
		ID       flexInt     `json:"id"`
		Amount   *flexString `json:"amount"`
		Category *string     `json:"category"`
		Date     *string     `json:"date"`
		Comment  *string     `json:"comment"`
	}
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	if args.ID <= 0 {
		return nil, domain.Invalid("id", "нужен id траты — сначала найди её через find_expenses")
	}
	var patch expense.Patch
	if args.Amount != nil {
		m, err := parseAmount(string(*args.Amount))
		if err != nil {
			return nil, err
		}
		patch.Amount = &m
	}
	if args.Category != nil {
		c, err := domain.ParseCategory(*args.Category)
		if err != nil {
			return nil, err
		}
		patch.Category = &c
	}
	if args.Date != nil {
		d, err := domain.ParseDate(*args.Date)
		if err != nil {
			return nil, err
		}
		patch.SpentAt = &d
	}
	patch.Comment = args.Comment
	if patch.Amount == nil && patch.Category == nil && patch.SpentAt == nil && patch.Comment == nil {
		return nil, domain.Invalid("fields", "не передано ни одного поля для изменения")
	}
	before, err := a.svc.Get(ctx, expense.Own(userID), int64(args.ID))
	if err != nil {
		return nil, err
	}
	e, err := a.svc.Update(ctx, expense.Own(userID), int64(args.ID), patch)
	if err != nil {
		return nil, err
	}
	return changeResult{payload: map[string]any{"updated": view(e)}, change: Change{Kind: ChangeUpdate, Expense: e, Before: &before}}, nil
}

func (a *Agent) deleteExpense(ctx context.Context, userID int64, raw []byte) (any, error) {
	var args struct {
		ID flexInt `json:"id"`
	}
	if err := decodeArgs(raw, &args); err != nil {
		return nil, err
	}
	if args.ID <= 0 {
		return nil, domain.Invalid("id", "нужен id траты — сначала найди её через find_expenses")
	}
	e, err := a.svc.Delete(ctx, expense.Own(userID), int64(args.ID))
	if err != nil {
		return nil, err
	}
	return changeResult{payload: map[string]any{"deleted": view(e)}, change: Change{Kind: ChangeDelete, Expense: e}}, nil
}

// resolveRange turns a period or from/to into dates. With required, a missing range means
// "this month" and a half-open range is closed with 2000-01-01 or today.
func (a *Agent) resolveRange(r rangeArgs, required bool) (*time.Time, *time.Time, error) {
	if r.Period != "" {
		if r.From != "" || r.To != "" {
			return nil, nil, domain.Invalid("period", "укажи либо period, либо from/to")
		}
		from, to, err := domain.ResolvePeriod(domain.Period(r.Period), a.svc.Now())
		if err != nil {
			return nil, nil, err
		}
		return &from, &to, nil
	}
	var from, to *time.Time
	if r.From != "" {
		t, err := domain.ParseDate(r.From)
		if err != nil {
			return nil, nil, err
		}
		from = &t
	}
	if r.To != "" {
		t, err := domain.ParseDate(r.To)
		if err != nil {
			return nil, nil, err
		}
		to = &t
	}
	if required {
		if from == nil && to == nil {
			f, t, _ := domain.ResolvePeriod(domain.PeriodThisMonth, a.svc.Now())
			return &f, &t, nil
		}
		if from == nil {
			f := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
			from = &f
		}
		if to == nil {
			t := a.svc.Today()
			to = &t
		}
	}
	return from, to, nil
}

type expenseView struct {
	ID         int64  `json:"id"`
	Amount     string `json:"amount"`
	Category   string `json:"category"`
	CategoryRU string `json:"category_ru"`
	Date       string `json:"date"`
	Weekday    string `json:"weekday"`
	Comment    string `json:"comment,omitempty"`
}

func view(e domain.Expense) expenseView {
	return expenseView{
		ID: e.ID, Amount: e.Amount.Human(), Category: string(e.Category), CategoryRU: e.Category.RU(),
		Date: domain.FormatDate(e.SpentAt), Weekday: domain.WeekdayRU(e.SpentAt), Comment: e.Comment,
	}
}

func parseAmount(s string) (domain.Money, error) {
	m, err := domain.ParseMoney(s)
	if err != nil {
		return 0, domain.Invalid("amount", fmt.Sprintf("некорректная сумма %q, нужны цифры", s))
	}
	return m, nil
}

func optionalCategory(s string) (*domain.Category, error) {
	if strings.TrimSpace(s) == "" {
		return nil, nil
	}
	c, err := domain.ParseCategory(s)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// toolError is the message the model sees; it never leaks internals.
func toolError(err error) string {
	var ve *domain.ValidationError
	var ae *argError
	switch {
	case errors.As(err, &ve):
		return ve.Message + " (" + ve.Field + ")"
	case errors.Is(err, domain.ErrNotFound):
		return "трата не найдена"
	case errors.As(err, &ae):
		return ae.Error()
	case errors.Is(err, errUnknownTool):
		return err.Error()
	default:
		return "внутренняя ошибка, попробуй позже"
	}
}

func isInternal(err error) bool {
	var ve *domain.ValidationError
	var ae *argError
	return !errors.As(err, &ve) && !errors.As(err, &ae) && !errors.Is(err, domain.ErrNotFound) && !errors.Is(err, errUnknownTool)
}
