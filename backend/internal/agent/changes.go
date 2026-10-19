package agent

import (
	"encoding/json"
	"errors"
	"regexp"
	"slices"
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

// composeReply builds the member's reply for a turn the model finished, from facts first:
// the confirmation of completed changes (from the tool results, never from the model's wording),
// then why calls of the last tool batch failed (distinct user-facing reasons), then the model's
// text with its own claims about changes removed. ErrEmptyReply means the model ended a turn
// that changed nothing without saying anything.
func composeReply(modelText string, changes []Change, failures []string, today time.Time) (string, error) {
	confirmation := confirmChanges(changes, today)
	modelText = strings.TrimSpace(dropRestatedChanges(modelText, confirmation))
	var parts []string
	if confirmation != "" {
		parts = append(parts, confirmation)
	}
	if len(failures) > 0 {
		parts = append(parts, "Не получилось: "+strings.Join(failures, "; ")+".")
	}
	if modelText != "" {
		parts = append(parts, modelText)
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

// confirmationStems start a word with which the model reports a change («Записал», «Записано»,
// «Добавлено», «Зафиксировал», «перенёс», «Готово»); lowercased, ё as е.
var confirmationStems = []string{"записал", "записан", "добав", "внес", "сохран", "измен", "исправ", "обнов", "помен", "перенес", "удал", "зафиксир", "учел", "учла", "готово"}

// hedgeWords mark a sentence that explains, qualifies or asks rather than reports.
var hedgeWords = map[string]bool{"но": true, "бы": true, "не": true, "однако": true}

// numberPattern matches numbers as the bot writes them, with thousands separated by spaces.
var numberPattern = regexp.MustCompile(`\d+(?:[ \x{00a0}\x{202f}]\d{3})*`)

// dropRestatedChanges removes the model's own sentences that report changes. Models do it
// despite the prompt, and the confirmation built from the tool results already says it
// correctly; with no changes such a sentence claims something that did not happen. A sentence
// that also carries a number the confirmation does not have is kept: it says something new.
func dropRestatedChanges(text, confirmation string) string {
	known := map[string]bool{}
	for _, n := range numbers(confirmation) {
		known[n] = true
	}
	var lines []string
	for line := range strings.Lines(text) {
		var kept []string
		for _, sentence := range splitSentences(line) {
			changed := confirmation != ""
			if !reportsChange(sentence, changed) || changed && hasNewNumber(sentence, known) {
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

// reportsChange tells whether a sentence reports a change, like «Записал кофе.», «Готово!» or,
// with anywhere, «Обед за 650 ₽ перенёс на вторник.». Without changes in the turn only a leading
// verb counts: «у тебя записано 5 трат» is then a fact, not a claim. A question or a sentence
// with «но», «бы», «не» explains or asks something and is not a report.
func reportsChange(sentence string, anywhere bool) bool {
	if strings.Contains(sentence, "?") {
		return false
	}
	words := strings.FieldsFunc(strings.ReplaceAll(strings.ToLower(sentence), "ё", "е"), func(r rune) bool { return !unicode.IsLetter(r) })
	if slices.ContainsFunc(words, func(w string) bool { return hedgeWords[w] }) {
		return false
	}
	isVerb := func(w string) bool {
		return slices.ContainsFunc(confirmationStems, func(stem string) bool { return strings.HasPrefix(w, stem) })
	}
	if anywhere {
		return slices.ContainsFunc(words, isVerb)
	}
	return len(words) > 0 && isVerb(words[0])
}

func hasNewNumber(sentence string, known map[string]bool) bool {
	return slices.ContainsFunc(numbers(sentence), func(n string) bool { return !known[n] })
}

// numbers lists the numbers in s with separators removed: "1 200 ₽, 14 октября" → 1200, 14.
func numbers(s string) []string {
	found := numberPattern.FindAllString(s, -1)
	for i, n := range found {
		found[i] = strings.Map(func(r rune) rune {
			if unicode.IsDigit(r) {
				return r
			}
			return -1
		}, n)
	}
	return found
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
