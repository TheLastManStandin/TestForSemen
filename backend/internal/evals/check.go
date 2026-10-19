package evals

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/agent"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

// Outcome is what one agent run produced.
type Outcome struct {
	Diff    Diff
	Reply   string
	Tools   []string
	Changes int // completed add/update/delete calls; an add then an update of it is one diff entry but two changes
}

var replyNormalizer = strings.NewReplacer("\u00a0", " ", "\u202f", " ", "ё", "е", "Ё", "е")

func normalize(s string) string { return strings.ToLower(replyNormalizer.Replace(s)) }

// Check compares an outcome with the expectation and returns failure reasons (empty = pass).
func Check(exp Expect, seeded map[string]domain.Expense, out Outcome, today time.Time) []string {
	var fails []string
	fail := func(format string, args ...any) { fails = append(fails, fmt.Sprintf(format, args...)) }

	// Created: exact count, each expectation matched by a distinct expense.
	if len(out.Diff.Created) != len(exp.Created) {
		fail("создано трат: %d, ожидалось %d (%s)", len(out.Diff.Created), len(exp.Created), describeAll(out.Diff.Created))
	} else {
		used := make([]bool, len(out.Diff.Created))
		for _, m := range exp.Created {
			found := false
			for i, e := range out.Diff.Created {
				if !used[i] && matchCreated(m, e, today) == nil {
					used[i], found = true, true
					break
				}
			}
			if !found {
				fail("нет траты %+v среди созданных: %s", m, describeAll(out.Diff.Created))
			}
		}
	}

	// Updated: by seed key.
	wantUpdated := map[int64]UpdateMatch{}
	for _, u := range exp.Updated {
		e, ok := seeded[u.Key]
		if !ok {
			fail("неизвестный ключ фикстуры %q", u.Key)
			continue
		}
		wantUpdated[e.ID] = u
	}
	if len(out.Diff.Updated) != len(exp.Updated) {
		fail("изменено трат: %d, ожидалось %d", len(out.Diff.Updated), len(exp.Updated))
	}
	for _, ch := range out.Diff.Updated {
		u, ok := wantUpdated[ch.After.ID]
		if !ok {
			fail("неожиданно изменена трата %s", describe(ch.Before))
			continue
		}
		if err := matchUpdated(u, ch.Before, ch.After, today); err != nil {
			fail("трата %s: %v (стало: %s)", u.Key, err, describe(ch.After))
		}
	}

	// Deleted: exact set of seed keys.
	wantDeleted := map[int64]string{}
	for _, key := range exp.Deleted {
		e, ok := seeded[key]
		if !ok {
			fail("неизвестный ключ фикстуры %q", key)
			continue
		}
		wantDeleted[e.ID] = key
	}
	gotDeleted := map[int64]bool{}
	for _, e := range out.Diff.Deleted {
		gotDeleted[e.ID] = true
		if _, ok := wantDeleted[e.ID]; !ok {
			fail("неожиданно удалена трата %s", describe(e))
		}
	}
	for id, key := range wantDeleted {
		if !gotDeleted[id] {
			fail("не удалена трата %s", key)
		}
	}

	// Reply.
	reply := normalize(out.Reply)
	if exp.Ask && !strings.Contains(reply, "?") {
		fail("ожидался уточняющий вопрос")
	}
	for _, s := range exp.ReplyContains {
		if !strings.Contains(reply, normalize(s)) {
			fail("в ответе нет %q", s)
		}
	}
	for _, s := range exp.ReplyNotContains {
		if strings.Contains(reply, normalize(s)) {
			fail("в ответе не должно быть %q", s)
		}
	}
	// The agent confirms each change with one line; more lines mean a restatement got through.
	if n := confirmationLines(out.Reply); out.Changes > 0 && n > out.Changes {
		fail("подтверждение изменений повторено: %d строк на %d изменений", n, out.Changes)
	}
	for _, tool := range exp.Tools {
		if !slices.Contains(out.Tools, tool) {
			fail("не вызван инструмент %s", tool)
		}
	}
	return fails
}

// confirmationLines counts reply lines that only confirm a change, as the agent judges it.
func confirmationLines(reply string) int {
	n := 0
	for line := range strings.Lines(reply) {
		if agent.RestatesChange(line) {
			n++
		}
	}
	return n
}

func matchCreated(m ExpenseMatch, e domain.Expense, today time.Time) error {
	amount, err := domain.ParseMoney(m.Amount)
	if err != nil {
		return err
	}
	if e.Amount != amount {
		return fmt.Errorf("сумма %s", e.Amount.Human())
	}
	if m.Category != "" && string(e.Category) != m.Category {
		return fmt.Errorf("категория %s", e.Category)
	}
	if m.Date != "" {
		d, err := ResolveDate(m.Date, today)
		if err != nil {
			return err
		}
		if !e.SpentAt.Equal(d) {
			return fmt.Errorf("дата %s", domain.FormatDate(e.SpentAt))
		}
	}
	if m.Comment != "" && !strings.Contains(normalize(e.Comment), normalize(m.Comment)) {
		return fmt.Errorf("комментарий %q", e.Comment)
	}
	return nil
}

// matchUpdated requires the listed fields to hold the expected values and every other
// field (and the owner) to stay as it was.
func matchUpdated(u UpdateMatch, before, after domain.Expense, today time.Time) error {
	if after.UserID != before.UserID {
		return fmt.Errorf("изменён владелец траты")
	}
	wantAmount := before.Amount
	if u.Amount != "" {
		var err error
		if wantAmount, err = domain.ParseMoney(u.Amount); err != nil {
			return err
		}
	}
	if after.Amount != wantAmount {
		return fmt.Errorf("сумма %s, ожидалось %s", after.Amount.Human(), wantAmount.Human())
	}
	wantCategory := string(before.Category)
	if u.Category != "" {
		wantCategory = u.Category
	}
	if string(after.Category) != wantCategory {
		return fmt.Errorf("категория %s, ожидалось %s", after.Category, wantCategory)
	}
	wantDate := before.SpentAt
	if u.Date != "" {
		var err error
		if wantDate, err = ResolveDate(u.Date, today); err != nil {
			return err
		}
	}
	if !after.SpentAt.Equal(wantDate) {
		return fmt.Errorf("дата %s, ожидалось %s", domain.FormatDate(after.SpentAt), domain.FormatDate(wantDate))
	}
	if u.Comment != "" {
		if !strings.Contains(normalize(after.Comment), normalize(u.Comment)) {
			return fmt.Errorf("комментарий %q, ожидалось %q", after.Comment, u.Comment)
		}
	} else if after.Comment != before.Comment {
		return fmt.Errorf("комментарий изменён: %q", after.Comment)
	}
	return nil
}

func describe(e domain.Expense) string {
	return fmt.Sprintf("[%s %s %s %q]", domain.FormatDate(e.SpentAt), e.Amount.Human(), e.Category, e.Comment)
}

func describeAll(es []domain.Expense) string {
	if len(es) == 0 {
		return "нет"
	}
	parts := make([]string, 0, len(es))
	for _, e := range es {
		parts = append(parts, describe(e))
	}
	return strings.Join(parts, ", ")
}
