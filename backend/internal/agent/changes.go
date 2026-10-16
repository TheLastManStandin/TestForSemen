package agent

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

// ErrPartial marks a turn that failed after it had already changed data. The error also
// wraps the real cause, and Result.Reply then holds a factual summary of the changes
// to send to the member instead of a generic error.
var ErrPartial = errors.New("agent: turn failed after completed changes")

// ChangeKind is the kind of a completed data change.
type ChangeKind string

const (
	ChangeAdd    ChangeKind = "add"
	ChangeUpdate ChangeKind = "update"
	ChangeDelete ChangeKind = "delete"
)

// Change is one successful add/update/delete made during a turn.
type Change struct {
	Kind    ChangeKind
	Expense domain.Expense  // the expense after the change (as deleted, for ChangeDelete)
	Before  *domain.Expense // the expense before an update; nil otherwise
	Patched PatchedFields   // fields the update set itself; zero otherwise
}

// PatchedFields marks which fields an update_expense call set, so changes made
// concurrently by someone else (e.g. the panel) are not attributed to the bot.
type PatchedFields struct {
	Comment, Amount, Category, Date bool
}

// replyPartialHeader opens the summary sent when a turn fails after changing data.
const replyPartialHeader = "Не удалось закончить ответ, но действия выполнены:"

// changeResult is a tool result that also reports the change it made.
// It marshals as the payload the model sees.
type changeResult struct {
	payload any
	change  Change
}

func (r changeResult) MarshalJSON() ([]byte, error) { return json.Marshal(r.payload) }

// summarizeChanges renders completed changes in Russian from the structured tool results,
// one line per change and without deduplication (two identical purchases are legitimate).
func summarizeChanges(changes []Change, today time.Time) string {
	var b strings.Builder
	b.WriteString(replyPartialHeader)
	for _, c := range changes {
		b.WriteString("\n• ")
		switch c.Kind {
		case ChangeAdd:
			b.WriteString("записал: " + describeExpense(c.Expense, today))
		case ChangeUpdate:
			b.WriteString("изменил: " + describeExpense(c.Expense, today))
			if c.Before != nil {
				if was := changedFrom(*c.Before, c.Expense, c.Patched, today); was != "" {
					b.WriteString(" (было " + was + ")")
				}
			}
		case ChangeDelete:
			b.WriteString("удалил: " + describeExpense(c.Expense, today))
		}
	}
	return b.String()
}

// describeExpense renders "кофе — 350 ₽, еда, 16 октября".
func describeExpense(e domain.Expense, today time.Time) string {
	s := e.Amount.Human() + ", " + e.Category.RU() + ", " + domain.HumanDate(e.SpentAt, today)
	if e.Comment != "" {
		s = e.Comment + " — " + s
	}
	return s
}

// changedFrom lists the previous values of the fields the update itself patched and changed.
func changedFrom(before, after domain.Expense, patched PatchedFields, today time.Time) string {
	var was []string
	if patched.Comment && before.Comment != after.Comment {
		if before.Comment == "" {
			was = append(was, "без комментария")
		} else {
			was = append(was, before.Comment)
		}
	}
	if patched.Amount && before.Amount != after.Amount {
		was = append(was, before.Amount.Human())
	}
	if patched.Category && before.Category != after.Category {
		was = append(was, before.Category.RU())
	}
	if patched.Date && !before.SpentAt.Equal(after.SpentAt) {
		was = append(was, domain.HumanDate(before.SpentAt, today))
	}
	return strings.Join(was, ", ")
}
