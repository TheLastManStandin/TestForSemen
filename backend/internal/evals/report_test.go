package evals

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/llm"
)

func sampleResults() []Result {
	return []Result{
		{Case: Case{Name: "a1", Group: "add", Input: "кофе 350"}, Latency: time.Second, Usage: llm.Usage{PromptTokens: 100, CompletionTokens: 10, Cost: 0.001}},
		{Case: Case{Name: "a2", Group: "add", Input: "купил кофе"}, Failures: []string{"ожидался уточняющий вопрос"}, Reply: "Записал", Tools: []string{"add_expense"}, Latency: 3 * time.Second},
		{Case: Case{Name: "q1", Group: "query", Input: "сколько"}, Err: "llm: timeout", Latency: 2 * time.Second},
	}
}

func TestSummarize(t *testing.T) {
	s := Summarize("vendor/model", sampleResults())
	assert.Equal(t, 3, s.Total)
	assert.Equal(t, 1, s.Passed)
	assert.InDelta(t, 33.3, s.PassRate(), 0.1)
	assert.Equal(t, 2*time.Second, s.AvgLatency)
	assert.Equal(t, []GroupStat{{Group: "add", Total: 2, Passed: 1}, {Group: "query", Total: 1, Passed: 0}}, s.Groups)
}

func TestRenderReport(t *testing.T) {
	r := RenderReport(Summarize("vendor/model", sampleResults()), sampleResults(), time.Date(2026, 10, 16, 14, 3, 0, 0, time.UTC))
	assert.Contains(t, r, "# Evals: `vendor/model`")
	assert.Contains(t, r, "**1/3 (33.3%)**")
	assert.Contains(t, r, "| add | 1/2 | 50.0% |")
	assert.Contains(t, r, "### a2 (add)")
	assert.Contains(t, r, "ожидался уточняющий вопрос")
	assert.Contains(t, r, "llm: timeout")
	assert.NotContains(t, r, "### a1")
}

func TestUpdateSummaryFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	require.NoError(t, UpdateSummaryFile(path, Summary{Model: "m/one", Total: 10, Passed: 9, AvgLatency: time.Second}))
	require.NoError(t, UpdateSummaryFile(path, Summary{Model: "m/two", Total: 10, Passed: 7, AvgLatency: 2 * time.Second}))
	require.NoError(t, UpdateSummaryFile(path, Summary{Model: "m/one", Total: 10, Passed: 10, AvgLatency: time.Second}))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(data)
	assert.Equal(t, 1, strings.Count(text, "| `m/one` |"), "rows are replaced, not duplicated")
	assert.Contains(t, text, "| `m/one` | 100.0% (10/10) |")
	assert.Contains(t, text, "| `m/two` | 70.0% (7/10) |")
	assert.Less(t, strings.Index(text, "m/one"), strings.Index(text, "m/two"), "sorted by pass rate")
}

func TestUpdateSummaryFileTieBreak(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	s := func(m string, cost float64, lat time.Duration) Summary {
		return Summary{Model: m, Total: 10, Passed: 10, AvgLatency: lat, Cost: cost}
	}
	require.NoError(t, UpdateSummaryFile(path, s("m/pricey", 0.14, time.Second)))
	require.NoError(t, UpdateSummaryFile(path, s("m/slow", 0.01, 5*time.Second)))
	require.NoError(t, UpdateSummaryFile(path, s("m/fast", 0.01, 2*time.Second)))
	require.NoError(t, UpdateSummaryFile(path, Summary{Model: "m/worse", Total: 10, Passed: 9, Cost: 0.001}))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(data)
	order := []string{"m/fast", "m/slow", "m/pricey", "m/worse"}
	for i := 1; i < len(order); i++ {
		assert.Less(t, strings.Index(text, order[i-1]), strings.Index(text, order[i]), "%s before %s", order[i-1], order[i])
	}
}

func TestSlug(t *testing.T) {
	assert.Equal(t, "anthropic_claude-haiku-5.5", Slug("anthropic/claude-haiku-5.5"))
	assert.Equal(t, "x_y_free", Slug("x/y:free"))
}
