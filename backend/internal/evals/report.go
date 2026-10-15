package evals

import (
	"fmt"
	"os"
	"sort"
	"strings"
	"time"
)

// GroupStat is the pass count of one case group.
type GroupStat struct {
	Group  string
	Total  int
	Passed int
}

// Summary aggregates a run.
type Summary struct {
	Model            string
	Total, Passed    int
	Groups           []GroupStat
	AvgLatency       time.Duration
	PromptTokens     int
	CompletionTokens int
	Cost             float64
}

// PassRate is a percentage.
func (s Summary) PassRate() float64 {
	if s.Total == 0 {
		return 0
	}
	return float64(s.Passed) * 100 / float64(s.Total)
}

// Summarize aggregates results by group.
func Summarize(model string, results []Result) Summary {
	s := Summary{Model: model, Total: len(results)}
	groups := map[string]*GroupStat{}
	var latency time.Duration
	for _, r := range results {
		g, ok := groups[r.Case.Group]
		if !ok {
			g = &GroupStat{Group: r.Case.Group}
			groups[r.Case.Group] = g
		}
		g.Total++
		if r.Passed() {
			g.Passed++
			s.Passed++
		}
		latency += r.Latency
		s.PromptTokens += r.Usage.PromptTokens
		s.CompletionTokens += r.Usage.CompletionTokens
		s.Cost += r.Usage.Cost
	}
	if len(results) > 0 {
		s.AvgLatency = latency / time.Duration(len(results))
	}
	for _, g := range groups {
		s.Groups = append(s.Groups, *g)
	}
	sort.Slice(s.Groups, func(i, j int) bool { return s.Groups[i].Group < s.Groups[j].Group })
	return s
}

// RenderReport renders a Markdown report with every failure explained.
func RenderReport(s Summary, results []Result, at time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Evals: `%s`\n\n", s.Model)
	fmt.Fprintf(&b, "- Дата: %s\n", at.Format("2006-01-02 15:04"))
	fmt.Fprintf(&b, "- Результат: **%d/%d (%.1f%%)**\n", s.Passed, s.Total, s.PassRate())
	fmt.Fprintf(&b, "- Средняя задержка: %.1f с · токены: %d / %d · стоимость: $%.4f\n\n", s.AvgLatency.Seconds(), s.PromptTokens, s.CompletionTokens, s.Cost)

	b.WriteString("| Группа | Пройдено | % |\n|---|---|---|\n")
	for _, g := range s.Groups {
		fmt.Fprintf(&b, "| %s | %d/%d | %.1f%% |\n", g.Group, g.Passed, g.Total, float64(g.Passed)*100/float64(g.Total))
	}

	var failed []Result
	for _, r := range results {
		if !r.Passed() {
			failed = append(failed, r)
		}
	}
	if len(failed) == 0 {
		b.WriteString("\nВсе кейсы пройдены.\n")
		return b.String()
	}
	b.WriteString("\n## Провалы\n")
	for _, r := range failed {
		fmt.Fprintf(&b, "\n### %s (%s)\n\n", r.Case.Name, r.Case.Group)
		fmt.Fprintf(&b, "- Ввод: «%s»\n", r.Case.Input)
		if r.Err != "" {
			fmt.Fprintf(&b, "- Ошибка: `%s`\n", r.Err)
			continue
		}
		fmt.Fprintf(&b, "- Ответ: «%s»\n", strings.ReplaceAll(r.Reply, "\n", " "))
		fmt.Fprintf(&b, "- Инструменты: %s\n", strings.Join(r.Tools, ", "))
		b.WriteString("- Причины:\n")
		for _, f := range r.Failures {
			fmt.Fprintf(&b, "  - %s\n", f)
		}
	}
	return b.String()
}

const summaryHeader = "# Сравнение моделей\n\nГенерируется `make eval`. Модель по умолчанию — лучшая по проценту прохождения, затем по стоимости и задержке.\n\n| Модель | Пройдено | Задержка | Стоимость прогона |\n|---|---|---|---|\n"

// UpdateSummaryFile inserts or replaces the model's row in the comparison table, sorted by pass rate.
func UpdateSummaryFile(path string, s Summary) error {
	rows := map[string]string{}
	rates := map[string]float64{}
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "| `") {
				continue
			}
			model := strings.SplitN(strings.TrimPrefix(line, "| `"), "`", 2)[0]
			rows[model] = line
			var passed, total int
			if _, err := fmt.Sscanf(strings.Split(line, "(")[1], "%d/%d", &passed, &total); err == nil && total > 0 {
				rates[model] = float64(passed) / float64(total)
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	rows[s.Model] = fmt.Sprintf("| `%s` | %.1f%% (%d/%d) | %.1f с | $%.4f |", s.Model, s.PassRate(), s.Passed, s.Total, s.AvgLatency.Seconds(), s.Cost)
	rates[s.Model] = s.PassRate() / 100

	models := make([]string, 0, len(rows))
	for m := range rows {
		models = append(models, m)
	}
	sort.Slice(models, func(i, j int) bool {
		if rates[models[i]] != rates[models[j]] {
			return rates[models[i]] > rates[models[j]]
		}
		return models[i] < models[j]
	})
	var b strings.Builder
	b.WriteString(summaryHeader)
	for _, m := range models {
		b.WriteString(rows[m] + "\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// Slug turns a model id into a file name.
func Slug(model string) string {
	return strings.NewReplacer("/", "_", ":", "_").Replace(model)
}
