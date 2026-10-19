package evals

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/llm"
)

func sampleResults() []Result {
	return []Result{
		{Case: Case{Name: "a1", Group: "add", Input: "кофе 350"}, Latency: time.Second, Usage: llm.Usage{PromptTokens: 100, CompletionTokens: 10, Cost: 0.001}},
		{Case: Case{Name: "a2", Group: "add", Input: "купил кофе"}, Failures: []string{"ожидался уточняющий вопрос"}, Reply: "Записал", Tools: []string{"add_expense"}, Latency: 3 * time.Second,
			Steps: []domain.TraceStep{{Tool: "add_expense", Args: json.RawMessage(`{"amount":"350"}`), Result: json.RawMessage(`{"error":"нужна категория"}`)}}},
		{Case: Case{Name: "q1", Group: "query", Input: "сколько"}, Err: "llm: timeout", Latency: 2 * time.Second},
	}
}

// withProviderErrors adds two cases the provider failed (429 and 502) to sampleResults.
func withProviderErrors() []Result {
	return append(sampleResults(),
		Result{Case: Case{Name: "q2", Group: "query", Input: "покажи траты"}, Err: "llm: llm api error 429: rate limited", ProviderError: true, Latency: 4 * time.Second},
		Result{Case: Case{Name: "a3", Group: "add", Input: "чай 100"}, Err: "llm: llm api error 502: bad gateway", ProviderError: true, Latency: 4 * time.Second},
	)
}

func TestSummarize(t *testing.T) {
	s := Summarize("vendor/model", sampleResults())
	assert.Equal(t, 3, s.Total)
	assert.Equal(t, 1, s.Passed)
	assert.InDelta(t, 33.3, s.PassRate(), 0.1)
	assert.Equal(t, 2*time.Second, s.AvgLatency)
	assert.Equal(t, 0, s.ProviderErrors)
	assert.Equal(t, []GroupStat{{Group: "add", Total: 2, Passed: 1}, {Group: "query", Total: 1, Passed: 0}}, s.Groups)
}

func TestSummarizeSeparatesProviderErrors(t *testing.T) {
	s := Summarize("vendor/model", withProviderErrors())
	assert.Equal(t, 5, s.Total)
	assert.Equal(t, 1, s.Passed)
	assert.Equal(t, 2, s.ProviderErrors)
	assert.Equal(t, 3, s.Functional())
	assert.InDelta(t, 33.3, s.PassRate(), 0.1, "pass rate ignores provider errors")
	assert.InDelta(t, 20.0, s.RawPassRate(), 0.1)
	assert.Equal(t, []GroupStat{{Group: "add", Total: 3, Passed: 1, ProviderErrors: 1}, {Group: "query", Total: 2, Passed: 0, ProviderErrors: 1}}, s.Groups)
}

func TestRenderReportSeparatesProviderErrors(t *testing.T) {
	results := withProviderErrors()
	r := RenderReport(Summarize("vendor/model", results), results, time.Date(2026, 10, 16, 14, 3, 0, 0, time.UTC))
	assert.Contains(t, r, "**1/3 (33.3%)**")
	assert.Contains(t, r, "без учёта ошибок провайдера; всего 1/5 (20.0%)")
	assert.Contains(t, r, "- Ошибки провайдера: 2")
	assert.Contains(t, r, "| add | 1/2 | 50.0% | 1 |")
	assert.Contains(t, r, "## Ошибки провайдера")
	_, rest, ok := strings.Cut(r, "## Провалы")
	require.True(t, ok)
	failures, provider, ok := strings.Cut(rest, "## Ошибки провайдера")
	require.True(t, ok)
	assert.Contains(t, failures, "### q1 (query)", "a non-provider error is a functional failure")
	assert.NotContains(t, failures, "### q2")
	assert.Contains(t, provider, "### q2 (query)")
	assert.Contains(t, provider, "llm api error 429")
}

func TestRenderReportListsNotesApartFromFailures(t *testing.T) {
	results := []Result{{Case: Case{Name: "n1", Group: "add", Input: "кофе 350"}, Reply: "Записал: кофе — 350 ₽\n\nЗаписано: 350 ₽", Notes: []string{"сумма 350 упомянута 2 раз(а)"}}}
	r := RenderReport(Summarize("vendor/model", results), results, time.Date(2026, 10, 16, 14, 3, 0, 0, time.UTC))
	assert.Contains(t, r, "**1/1 (100.0%)**", "a note does not fail the case")
	assert.NotContains(t, r, "## Провалы")
	_, notes, ok := strings.Cut(r, "## Замечания")
	require.True(t, ok)
	assert.Contains(t, notes, "### n1 (add)")
	assert.Contains(t, notes, "сумма 350 упомянута 2 раз(а)")
}

func TestRenderReport(t *testing.T) {
	r := RenderReport(Summarize("vendor/model", sampleResults()), sampleResults(), time.Date(2026, 10, 16, 14, 3, 0, 0, time.UTC))
	assert.Contains(t, r, "# Evals: `vendor/model`")
	assert.Contains(t, r, "**1/3 (33.3%)**")
	assert.Contains(t, r, "| add | 1/2 | 50.0% | 0 |")
	assert.Contains(t, r, "### a2 (add)")
	assert.Contains(t, r, "ожидался уточняющий вопрос")
	assert.Contains(t, r, "llm: timeout")
	assert.NotContains(t, r, "### a1")
	assert.Contains(t, r, "1. `add_expense` `{\"amount\":\"350\"}` → `{\"error\":\"нужна категория\"}`", "failed cases show raw tool calls")
}

func TestUpdateSummaryFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	require.NoError(t, UpdateSummaryFile(path, &Summary{Model: "m/one", Total: 10, Passed: 9, AvgLatency: time.Second}, "runs/r1.md"))
	require.NoError(t, UpdateSummaryFile(path, &Summary{Model: "m/two", Total: 10, Passed: 7, AvgLatency: 2 * time.Second}, "runs/r2.md"))
	require.NoError(t, UpdateSummaryFile(path, &Summary{Model: "m/one", Total: 10, Passed: 9, ProviderErrors: 1, AvgLatency: time.Second}, "runs/r3.md"))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(data)
	assert.Equal(t, 1, strings.Count(text, "| `m/one` |"), "rows are replaced, not duplicated")
	assert.Contains(t, text, "| `m/one` | 100.0% (9/9) | 1 (неполный прогон) | 90.0% (9/10) | 1.0 с | $0.0000 | [отчёт](runs/r3.md) |")
	assert.Contains(t, text, "| `m/two` | 70.0% (7/10) | 0 | 70.0% (7/10) | 2.0 с | $0.0000 | [отчёт](runs/r2.md) |")
	assert.Less(t, strings.Index(text, "m/one"), strings.Index(text, "m/two"), "sorted by pass rate")
}

func TestUpdateSummaryFileMigratesOldRows(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	old := "# Сравнение моделей\n\n| Модель | Пройдено | Задержка | Стоимость прогона |\n|---|---|---|---|\n" +
		"| `m/old` | 100.0% (45/45) | 1.9 с | $0.0101 |\n"
	require.NoError(t, os.WriteFile(path, []byte(old), 0o644))
	require.NoError(t, UpdateSummaryFile(path, &Summary{Model: "m/new", Total: 10, Passed: 5}, "runs/n.md"))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Contains(t, string(data), "| `m/old` | 100.0% (45/45) | — | 100.0% (45/45) | 1.9 с | $0.0101 | — |")
	assert.Less(t, strings.Index(string(data), "m/old"), strings.Index(string(data), "m/new"))
}

func TestUpdateSummaryFileListsRunHistory(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "README.md")
	require.NoError(t, UpdateSummaryFile(path, &Summary{Model: "m/one", Total: 10, Passed: 10}, "runs/2026-10-16_10-00-00_m_one.md"))
	before, err := os.ReadFile(path)
	require.NoError(t, err)

	runs := filepath.Join(dir, "runs")
	require.NoError(t, os.MkdirAll(runs, 0o755))
	for _, f := range []string{"2026-10-16_10-00-00_m_one.md", "2026-10-16_11-30-00_m_one_update.md", "notes.txt"} {
		require.NoError(t, os.WriteFile(filepath.Join(runs, f), []byte("x"), 0o644))
	}
	require.NoError(t, UpdateSummaryFile(path, nil, "")) // a filtered run: table untouched, history refreshed

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(data)
	table := func(s string) string {
		head, _, _ := strings.Cut(s, "## История прогонов")
		return strings.TrimSpace(head)
	}
	assert.Equal(t, table(string(before)), table(text), "a filtered run does not change the table")
	assert.Contains(t, text, "## История прогонов")
	assert.Contains(t, text, "- [2026-10-16_11-30-00_m_one_update.md](runs/2026-10-16_11-30-00_m_one_update.md)")
	assert.Less(t, strings.Index(text, "11-30-00"), strings.Index(text, "- [2026-10-16_10-00-00"), "newest first")
	assert.NotContains(t, text, "notes.txt")
}

func TestUpdateSummaryFileRanksByRawRateAndFlagsIncompleteRuns(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	// 20/20 functional but 25 cases lost to the provider must not outrank a clean 44/45.
	require.NoError(t, UpdateSummaryFile(path, &Summary{Model: "m/flaky", Total: 45, Passed: 20, ProviderErrors: 25}, ""))
	require.NoError(t, UpdateSummaryFile(path, &Summary{Model: "m/clean", Total: 45, Passed: 44, Cost: 1}, ""))
	// Equal raw rate: the higher functional rate wins, then cost.
	require.NoError(t, UpdateSummaryFile(path, &Summary{Model: "m/tie-cheap", Total: 10, Passed: 8, Cost: 0.1}, ""))
	require.NoError(t, UpdateSummaryFile(path, &Summary{Model: "m/tie-func", Total: 10, Passed: 8, ProviderErrors: 1, Cost: 0.5}, ""))

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	text := string(data)
	order := []string{"m/clean", "m/tie-func", "m/tie-cheap", "m/flaky"}
	for i := 1; i < len(order); i++ {
		assert.Less(t, strings.Index(text, order[i-1]), strings.Index(text, order[i]), "%s before %s", order[i-1], order[i])
	}
	assert.Contains(t, text, "| `m/flaky` | 100.0% (20/20) | 25 (неполный прогон) | 44.4% (20/45) |")
	assert.Contains(t, text, "| `m/clean` | 97.8% (44/45) | 0 | 97.8% (44/45) |")
	assert.Contains(t, text, "сначала по проценту от всех кейсов")
	assert.Contains(t, text, "не может быть моделью по умолчанию")

	// The flag and provider count survive a rewrite.
	require.NoError(t, UpdateSummaryFile(path, nil, ""))
	again, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, text, string(again))
}

func TestUpdateSummaryFileTieBreak(t *testing.T) {
	path := filepath.Join(t.TempDir(), "README.md")
	s := func(m string, cost float64, lat time.Duration) *Summary {
		return &Summary{Model: m, Total: 10, Passed: 10, AvgLatency: lat, Cost: cost}
	}
	require.NoError(t, UpdateSummaryFile(path, s("m/pricey", 0.14, time.Second), ""))
	require.NoError(t, UpdateSummaryFile(path, s("m/slow", 0.01, 5*time.Second), ""))
	require.NoError(t, UpdateSummaryFile(path, s("m/fast", 0.01, 2*time.Second), ""))
	require.NoError(t, UpdateSummaryFile(path, &Summary{Model: "m/worse", Total: 10, Passed: 9, Cost: 0.001}, ""))

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
