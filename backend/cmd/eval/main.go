// Command eval runs the agent eval suite against a real LLM and writes Markdown reports.
package main

import (
	"cmp"
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"time"

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

// checkWritable decides whether a finished run may overwrite reports: not when it
// was interrupted, and not when every case failed on infrastructure (provider down).
func checkWritable(ctxErr error, results []evals.Result) error {
	if ctxErr != nil {
		return fmt.Errorf("run interrupted, reports not written: %w", ctxErr)
	}
	for _, r := range results {
		if r.Err == "" {
			return nil
		}
	}
	return fmt.Errorf("every case failed with an infrastructure error (first: %s), reports not written", results[0].Err)
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
	if err := checkWritable(ctx.Err(), results); err != nil {
		return err
	}
	summary := evals.Summarize(model, results)

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return err
	}
	reportPath := filepath.Join(outDir, reportFileName(model, filter))
	if err := os.WriteFile(reportPath, []byte(evals.RenderReport(summary, results, time.Now())), 0o644); err != nil {
		return err
	}
	if filter == "" {
		if err := evals.UpdateSummaryFile(filepath.Join(outDir, "README.md"), summary); err != nil {
			return err
		}
	}

	for _, r := range results {
		if !r.Passed() {
			fmt.Printf("  FAIL %-32s %s%v\n", r.Case.Name, r.Err, r.Failures)
		}
	}
	fmt.Printf("%s: %d/%d passed (%.1f%%), avg %.1fs, $%.4f, took %s → %s\n",
		model, summary.Passed, summary.Total, summary.PassRate(), summary.AvgLatency.Seconds(), summary.Cost,
		time.Since(started).Round(time.Second), reportPath)
	return nil
}
