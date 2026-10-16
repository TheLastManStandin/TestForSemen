package evals

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

// GroupStat is the pass count of one case group.
type GroupStat struct {
	Group          string
	Total          int
	Passed         int
	ProviderErrors int
}

// Summary aggregates a run.
type Summary struct {
	Model            string
	Total, Passed    int
	ProviderErrors   int // cases lost to the provider (429, 5xx, timeouts); not the agent's fault
	Groups           []GroupStat
	AvgLatency       time.Duration
	PromptTokens     int
	CompletionTokens int
	Cost             float64
}

// Functional is the number of cases that produced an outcome attributable to the agent.
func (s Summary) Functional() int { return s.Total - s.ProviderErrors }

// PassRate is the percentage of functional cases that passed.
func (s Summary) PassRate() float64 { return percent(s.Passed, s.Functional()) }

// RawPassRate is the percentage of all cases that passed, provider errors included.
func (s Summary) RawPassRate() float64 { return percent(s.Passed, s.Total) }

func percent(n, of int) float64 {
	if of == 0 {
		return 0
	}
	return float64(n) * 100 / float64(of)
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
		switch {
		case r.Passed():
			g.Passed++
			s.Passed++
		case r.ProviderError:
			g.ProviderErrors++
			s.ProviderErrors++
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

// RenderReport renders a Markdown report with every failure explained. Provider errors
// are listed apart from functional failures and excluded from the pass rate.
func RenderReport(s Summary, results []Result, at time.Time) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Evals: `%s`\n\n", s.Model)
	fmt.Fprintf(&b, "- Дата: %s\n", at.Format("2006-01-02 15:04"))
	fmt.Fprintf(&b, "- Результат: **%d/%d (%.1f%%)** без учёта ошибок провайдера; всего %d/%d (%.1f%%)\n",
		s.Passed, s.Functional(), s.PassRate(), s.Passed, s.Total, s.RawPassRate())
	fmt.Fprintf(&b, "- Ошибки провайдера: %d\n", s.ProviderErrors)
	fmt.Fprintf(&b, "- Средняя задержка: %.1f с · токены: %d / %d · стоимость: $%.4f\n\n", s.AvgLatency.Seconds(), s.PromptTokens, s.CompletionTokens, s.Cost)

	b.WriteString("| Группа | Пройдено | % | Ошибки провайдера |\n|---|---|---|---|\n")
	for _, g := range s.Groups {
		functional := g.Total - g.ProviderErrors
		fmt.Fprintf(&b, "| %s | %d/%d | %.1f%% | %d |\n", g.Group, g.Passed, functional, percent(g.Passed, functional), g.ProviderErrors)
	}

	var failed, provider []Result
	for _, r := range results {
		switch {
		case r.Passed():
		case r.ProviderError:
			provider = append(provider, r)
		default:
			failed = append(failed, r)
		}
	}
	if len(failed) == 0 && len(provider) == 0 {
		b.WriteString("\nВсе кейсы пройдены.\n")
		return b.String()
	}
	if len(failed) > 0 {
		b.WriteString("\n## Провалы\n")
		for _, r := range failed {
			renderFailure(&b, r)
		}
	}
	if len(provider) > 0 {
		b.WriteString("\n## Ошибки провайдера\n\nНе считаются провалами агента; кейсы стоит перезапустить.\n")
		for _, r := range provider {
			renderFailure(&b, r)
		}
	}
	return b.String()
}

func renderFailure(b *strings.Builder, r Result) {
	fmt.Fprintf(b, "\n### %s (%s)\n\n", r.Case.Name, r.Case.Group)
	fmt.Fprintf(b, "- Ввод: «%s»\n", r.Case.Input)
	if r.Err != "" {
		fmt.Fprintf(b, "- Ошибка: `%s`\n", r.Err)
	} else {
		fmt.Fprintf(b, "- Ответ: «%s»\n", strings.ReplaceAll(r.Reply, "\n", " "))
		fmt.Fprintf(b, "- Инструменты: %s\n", strings.Join(r.Tools, ", "))
		b.WriteString("- Причины:\n")
		for _, f := range r.Failures {
			fmt.Fprintf(b, "  - %s\n", f)
		}
	}
	renderSteps(b, r.Steps)
}

// renderSteps lists the raw tool calls so a failure can be diagnosed from the report alone.
func renderSteps(b *strings.Builder, steps []domain.TraceStep) {
	if len(steps) == 0 {
		return
	}
	b.WriteString("- Вызовы:\n")
	for i, s := range steps {
		fmt.Fprintf(b, "  %d. `%s` `%s` → `%s`\n", i+1, s.Tool, s.Args, s.Result)
	}
}

// RunsDir is the directory (next to README.md) that keeps one dated record per run.
const RunsDir = "runs"

const summaryHeader = "# Сравнение моделей\n\n" +
	"Генерируется `make eval`. Строки отсортированы сначала по проценту от всех кейсов («Всего»), затем по «Пройдено», стоимости и задержке.\n" +
	"«Пройдено» считается без кейсов, упавших на ошибках провайдера (429, 5xx, таймауты); «Всего» — по всем кейсам.\n" +
	"Прогон с ошибками провайдера помечен «неполный прогон» и не может быть моделью по умолчанию: ею становится верхняя строка без этой пометки.\n" +
	"Строки со значением «—» в «Ошибки провайдера» перенесены из старого формата таблицы: их итоги могли включать инфраструктурные ошибки как провалы.\n\n" +
	"| Модель | Пройдено | Ошибки провайдера | Всего | Задержка | Стоимость прогона | Последний полный прогон |\n" +
	"|---|---|---|---|---|---|---|\n"

const historyHeader = "\n## История прогонов\n\nКаждый прогон, включая частичные (`-filter`) и повторные, сохраняется в `runs/` и не перезаписывается.\n\n"

// summaryRow is one parsed line of the comparison table.
type summaryRow struct {
	model              string
	passed, functional int
	total              int
	providerErrors     int // -1: unknown (row from the old table format)
	latency, cost      float64
	link               string // relative run record path, "" if none
}

// rawRate is the share of all cases that passed; ranking uses it first, so cases lost to
// the provider cannot lift a model above one that finished every case.
func (r summaryRow) rawRate() float64 { return percent(r.passed, r.total) }

func (r summaryRow) functionalRate() float64 { return percent(r.passed, r.functional) }

// incomplete marks a run with provider errors: it is shown but never the default choice.
const incomplete = "неполный прогон"

func (r summaryRow) render() string {
	provider, link := "—", "—"
	if r.providerErrors >= 0 {
		provider = fmt.Sprint(r.providerErrors)
	}
	if r.providerErrors > 0 {
		provider += " (" + incomplete + ")"
	}
	if r.link != "" {
		link = fmt.Sprintf("[отчёт](%s)", r.link)
	}
	return fmt.Sprintf("| `%s` | %.1f%% (%d/%d) | %s | %.1f%% (%d/%d) | %.1f с | $%.4f | %s |",
		r.model, percent(r.passed, r.functional), r.passed, r.functional, provider,
		percent(r.passed, r.total), r.passed, r.total, r.latency, r.cost, link)
}

// parseSummaryRow reads both the current 7-column and the old 4-column table format.
func parseSummaryRow(line string) (summaryRow, bool) {
	cells := strings.Split(line, "|")
	for i := range cells {
		cells[i] = strings.TrimSpace(cells[i])
	}
	var latCell, costCell string
	row := summaryRow{model: strings.Trim(cells[min(1, len(cells)-1)], "`"), providerErrors: -1}
	switch len(cells) {
	case 6: // | model | passed | latency | cost |
		latCell, costCell = cells[3], cells[4]
	case 9: // | model | passed | provider | total | latency | cost | link |
		latCell, costCell = cells[5], cells[6]
		if _, err := fmt.Sscanf(cells[3], "%d", &row.providerErrors); err != nil {
			row.providerErrors = -1
		}
		if i, j := strings.Index(cells[7], "]("), strings.LastIndex(cells[7], ")"); i >= 0 && j > i {
			row.link = cells[7][i+2 : j]
		}
	default:
		return summaryRow{}, false
	}
	if _, err := fmt.Sscanf(cells[2][strings.LastIndex(cells[2], "(")+1:], "%d/%d", &row.passed, &row.functional); err != nil {
		return summaryRow{}, false
	}
	row.total = row.functional
	if row.providerErrors > 0 {
		row.total += row.providerErrors
	}
	_, _ = fmt.Sscanf(latCell, "%f", &row.latency)
	_, _ = fmt.Sscanf(strings.TrimPrefix(costCell, "$"), "%f", &row.cost)
	return row, row.model != ""
}

// UpdateSummaryFile rewrites the comparison README at path. A full run passes its summary
// and run record (relative to the README) to insert or replace the model's row; a
// filtered run passes nil and only refreshes the run history. Rows are sorted by raw pass
// rate (desc), then functional pass rate (desc), cost (asc), latency (asc) and name.
func UpdateSummaryFile(path string, s *Summary, run string) error {
	rows := map[string]summaryRow{}
	if data, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			if !strings.HasPrefix(line, "| `") {
				continue
			}
			if row, ok := parseSummaryRow(line); ok {
				rows[row.model] = row
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	if s != nil {
		rows[s.Model] = summaryRow{
			model: s.Model, passed: s.Passed, functional: s.Functional(), total: s.Total, providerErrors: s.ProviderErrors,
			latency: s.AvgLatency.Seconds(), cost: s.Cost, link: run,
		}
	}

	sorted := make([]summaryRow, 0, len(rows))
	for _, r := range rows {
		sorted = append(sorted, r)
	}
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i], sorted[j]
		switch {
		case a.rawRate() != b.rawRate():
			return a.rawRate() > b.rawRate()
		case a.functionalRate() != b.functionalRate():
			return a.functionalRate() > b.functionalRate()
		case a.cost != b.cost:
			return a.cost < b.cost
		case a.latency != b.latency:
			return a.latency < b.latency
		}
		return a.model < b.model
	})
	var b strings.Builder
	b.WriteString(summaryHeader)
	for _, r := range sorted {
		b.WriteString(r.render() + "\n")
	}

	runs, err := runRecords(filepath.Join(filepath.Dir(path), RunsDir))
	if err != nil {
		return err
	}
	if len(runs) > 0 {
		b.WriteString(historyHeader)
		for _, name := range runs {
			fmt.Fprintf(&b, "- [%s](%s/%s)\n", name, RunsDir, name)
		}
	}
	return os.WriteFile(path, []byte(b.String()), 0o644)
}

// runRecords lists run record files, newest first (names start with a timestamp).
func runRecords(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".md") {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return names, nil
}

// Slug turns a model id into a file name.
func Slug(model string) string {
	return strings.NewReplacer("/", "_", ":", "_").Replace(model)
}
