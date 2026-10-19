package agent

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

// ErrPartial marks a turn that failed after it had already changed data. The error also
// wraps the real cause, and Result.Reply then holds a factual summary of the changes
// to send to the member instead of a generic error.
var ErrPartial = errors.New("agent: turn failed after completed changes")

// ErrEmptyReply is recorded in the trace when the model ends a turn that changed nothing
// without a reply. Run does not return it: the member still gets a reply.
var ErrEmptyReply = errors.New("agent: empty model reply")

// ReplyNoAnswer is sent when the model ends a turn with neither changes nor a reply.
const ReplyNoAnswer = "Не получилось ответить, попробуй сформулировать иначе"

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

// summarizeChanges closes a turn that failed after changing data.
func summarizeChanges(changes []Change, today time.Time) string {
	return replyPartialHeader + "\n" + confirmChanges(changes, today)
}

// confirmChanges renders completed changes in Russian from the structured tool results,
// one line per change and without deduplication (two identical purchases are legitimate).
func confirmChanges(changes []Change, today time.Time) string {
	lines := make([]string, 0, len(changes))
	for _, c := range changes {
		switch c.Kind {
		case ChangeAdd:
			lines = append(lines, "Записал: "+describeExpense(c.Expense, today))
		case ChangeUpdate:
			line := "Изменил: " + describeExpense(c.Expense, today)
			if c.Before != nil {
				if was := changedFrom(*c.Before, c.Expense, c.Patched, today); was != "" {
					line += " (было " + was + ")"
				}
			}
			lines = append(lines, line)
		case ChangeDelete:
			lines = append(lines, "Удалил: "+describeExpense(c.Expense, today))
		}
	}
	return strings.Join(lines, "\n")
}

// composeReply builds the member's reply for a turn the model finished. Changes are confirmed
// from the tool results, never from the model's wording; the model's text follows as the answer
// to the rest of the message. failures are the distinct user-facing reasons calls of the last tool
// batch failed. ErrEmptyReply means the model ended the turn without changing or saying anything.
func composeReply(modelText string, changes []Change, failures []string, today time.Time) (string, error) {
	var parts []string
	if len(changes) > 0 {
		parts = append(parts, confirmChanges(changes, today))
	}
	if len(changes) > 0 {
		modelText = dropRestatedConfirmations(modelText)
	}
	switch modelText = strings.TrimSpace(modelText); {
	case modelText != "":
		parts = append(parts, modelText)
	case len(failures) > 0:
		parts = append(parts, "Не получилось: "+strings.Join(failures, "; ")+".")
	}
	reply := strings.Join(parts, "\n\n")
	if len(changes) == 0 && modelText == "" {
		if reply == "" {
			reply = ReplyNoAnswer
		}
		return reply, ErrEmptyReply
	}
	return reply, nil
}

// confirmationVerbs open a sentence in which the model restates a change (compared lowercased, ё as е).
var confirmationVerbs = []string{"записал", "добавил", "внес", "сохранил", "изменил", "исправил", "обновил", "поменял", "удалил", "готово"}

// dropRestatedConfirmations removes the model's own sentences about changes: models restate them
// despite the prompt, and the confirmation built from the tool results already says it, correctly.
// The rest of the text (answers, questions) is kept.
func dropRestatedConfirmations(text string) string {
	var lines []string
	for line := range strings.Lines(text) {
		var kept []string
		for _, sentence := range splitSentences(line) {
			if !restatesChange(sentence) {
				kept = append(kept, sentence)
			}
		}
		if len(kept) > 0 {
			lines = append(lines, strings.Join(kept, " "))
		}
	}
	return strings.Join(lines, "\n")
}

// splitSentences cuts a line after ".", "!", "?" or "…" followed by a space.
func splitSentences(line string) []string {
	var out []string
	start := 0
	runes := []rune(line)
	for i, r := range runes {
		last := i == len(runes)-1
		if last || strings.ContainsRune(".!?…", r) && unicode.IsSpace(runes[i+1]) {
			if s := strings.TrimSpace(string(runes[start : i+1])); s != "" {
				out = append(out, s)
			}
			start = i + 1
		}
	}
	return out
}

func restatesChange(sentence string) bool {
	s := strings.ReplaceAll(strings.ToLower(strings.TrimLeft(sentence, " •-*✅")), "ё", "е")
	for _, verb := range confirmationVerbs {
		if strings.HasPrefix(s, verb) {
			return true
		}
	}
	return false
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
