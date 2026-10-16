// Command eval runs the agent eval suite against a real LLM and writes Markdown reports.
package main

import (
	"cmp"
	"context"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"
	"unicode"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/evals"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/llm"
)

func main() {
	model := flag.String("model", os.Getenv("LLM_MODEL"), "model id (default: $LLM_MODEL)")
	casesPath := flag.String("cases", "evals/cases.yaml", "cases file")
	outDir := flag.String("out", "evals/results", "report directory")
	parallel := flag.Int("parallel", 4, "cases run concurrently")
	filter := flag.String("filter", "", "run only cases whose name or group contains this")
	flag.Parse()

	if err := run(*model, *casesPath, *outDir, *parallel, *filter); err != nil {
		fmt.Fprintln(os.Stderr, "eval:", err)
		os.Exit(1)
	}
}

// reportFileName keeps a filtered run from overwriting the model's full report.
func reportFileName(model, filter string) string {
	if filter != "" {
		return evals.Slug(model) + ".partial.md"
	}
	return evals.Slug(model) + ".md"
}

// runFileName names a run record: start time, model and the filter, if any.
func runFileName(model, filter string, at time.Time) string {
	name := at.Format("2006-01-02_15-04-05") + "_" + evals.Slug(model)
	if filter != "" {
		name += "_" + strings.Map(func(r rune) rune {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '.' {
				return r
			}
			return '-'
		}, filter)
	}
	return name + ".md"
}

// writeRunRecord saves report as a new file in outDir/runs and returns its path relative
// to outDir. It never overwrites: a name clash gets a numeric suffix.
func writeRunRecord(outDir, model, filter string, at time.Time, report string) (string, error) {
	dir := filepath.Join(outDir, evals.RunsDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	base := strings.TrimSuffix(runFileName(model, filter, at), ".md")
	for i := 1; ; i++ {
		name := base + ".md"
		if i > 1 {
			name = fmt.Sprintf("%s_%d.md", base, i)
		}
		f, err := os.OpenFile(filepath.Join(dir, name), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o644)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		_, err = f.WriteString(report)
		if cErr := f.Close(); err == nil {
			err = cErr
		}
		return filepath.Join(evals.RunsDir, name), err
	}
}

// checkWritable decides whether a finished run may overwrite the model's report and
// table row: not when it was interrupted, and not when every case failed on
// infrastructure (provider down). Such a run still gets its own run record.
func checkWritable(ctxErr error, results []evals.Result) error {
	if ctxErr != nil {
		return fmt.Errorf("run interrupted, reports not written: %w", ctxErr)
	}
	for _, r := range results {
		if r.Err == "" {
			return nil
		}
	}
	return fmt.Errorf("every case failed with an infrastructure error (first: %s), model report not updated", results[0].Err)
}

func run(model, casesPath, outDir string, parallel int, filter string) error {
	if model == "" {
		return fmt.Errorf("set -model or LLM_MODEL")
	}
	apiKey := os.Getenv("LLM_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("LLM_API_KEY is not set")
	}
	cases, err := evals.LoadCases(casesPath)
	if err != nil {
		return err
	}
	cases = evals.Filter(cases, filter)
	if len(cases) == 0 {
		return fmt.Errorf("no cases match %q", filter)
	}

	client := llm.NewOpenAI(llm.OpenAIConfig{
		BaseURL: cmp.Or(os.Getenv("LLM_BASE_URL"), "https://openrouter.ai/api/v1"),
		APIKey:  apiKey, Timeout: 60 * time.Second, MaxRetries: 3, Title: "Family Expenses Evals",
	})
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()

	fmt.Printf("Running %d cases on %s…\n", len(cases), model)
	started := time.Now()
	results := (&evals.Runner{LLM: client, Model: model, Parallel: parallel}).Run(ctx, cases)
	if ctx.Err() != nil {
		return fmt.Errorf("run interrupted, reports not written: %w", ctx.Err())
	}
	summary := evals.Summarize(model, results)
	at := time.Now()
	report := evals.RenderReport(summary, results, at)

	// Every finished run is kept, even when the provider failed every case.
	runPath, err := writeRunRecord(outDir, model, filter, at, report)
	if err != nil {
		return err
	}
	writable := checkWritable(nil, results)
	var row *evals.Summary
	if writable == nil {
		if err := os.WriteFile(filepath.Join(outDir, reportFileName(model, filter)), []byte(report), 0o644); err != nil {
			return err
		}
		if filter == "" {
			row = &summary
		}
	}
	if err := evals.UpdateSummaryFile(filepath.Join(outDir, "README.md"), row, runPath); err != nil {
		return err
	}

	for _, r := range results {
		switch {
		case r.ProviderError:
			fmt.Printf("  PROVIDER %-28s %s\n", r.Case.Name, r.Err)
		case !r.Passed():
			fmt.Printf("  FAIL %-32s %s%v\n", r.Case.Name, r.Err, r.Failures)
		}
	}
	fmt.Printf("%s: %d/%d passed (%.1f%%), provider errors %d (raw %d/%d), avg %.1fs, $%.4f, took %s → %s\n",
		model, summary.Passed, summary.Functional(), summary.PassRate(), summary.ProviderErrors, summary.Passed, summary.Total,
		summary.AvgLatency.Seconds(), summary.Cost, time.Since(started).Round(time.Second), filepath.Join(outDir, runPath))
	return writable
}
