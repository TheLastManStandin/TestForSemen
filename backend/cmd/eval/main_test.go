package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/evals"
)

func TestReportFileName(t *testing.T) {
	cases := []struct {
		model, filter, want string
	}{
		{"anthropic/claude-haiku-5.5", "", "anthropic_claude-haiku-5.5.md"},
		{"anthropic/claude-haiku-5.5", "delete", "anthropic_claude-haiku-5.5.partial.md"},
	}
	for _, c := range cases {
		if got := reportFileName(c.model, c.filter); got != c.want {
			t.Errorf("reportFileName(%q, %q) = %q, want %q", c.model, c.filter, got, c.want)
		}
	}
}

func TestRunFileName(t *testing.T) {
	at := time.Date(2026, 10, 16, 14, 3, 5, 0, time.UTC)
	cases := []struct {
		model, filter, want string
	}{
		{"deepseek/deepseek-v4.1-flash", "", "2026-10-16_14-03-05_deepseek_deepseek-v4.1-flash.md"},
		{"deepseek/deepseek-v4.1-flash", "update_yesterday_taxi", "2026-10-16_14-03-05_deepseek_deepseek-v4.1-flash_update_yesterday_taxi.md"},
		{"x/y:free", "a b/c", "2026-10-16_14-03-05_x_y_free_a-b-c.md"},
	}
	for _, c := range cases {
		if got := runFileName(c.model, c.filter, at); got != c.want {
			t.Errorf("runFileName(%q, %q) = %q, want %q", c.model, c.filter, got, c.want)
		}
	}
}

func TestWriteRunRecordNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	at := time.Date(2026, 10, 16, 14, 3, 5, 0, time.UTC)
	first, err := writeRunRecord(dir, "m/x", "f", at, "one")
	if err != nil {
		t.Fatal(err)
	}
	second, err := writeRunRecord(dir, "m/x", "f", at, "two")
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatalf("same-second runs share %s", first)
	}
	for path, want := range map[string]string{first: "one", second: "two"} {
		got, err := os.ReadFile(filepath.Join(dir, path))
		if err != nil || string(got) != want {
			t.Errorf("%s = %q, %v; want %q", path, got, err, want)
		}
	}
	if filepath.Dir(first) != "runs" {
		t.Errorf("run records live in runs/, got %s", first)
	}
}

func TestCheckWritable(t *testing.T) {
	ok := evals.Result{}
	infra := evals.Result{Err: "provider down"}
	failed := evals.Result{Failures: []string{"wrong tool"}}

	if err := checkWritable(nil, []evals.Result{ok, infra}); err != nil {
		t.Errorf("mixed run: %v", err)
	}
	if err := checkWritable(nil, []evals.Result{failed, infra}); err != nil {
		t.Errorf("run with a real (non-infra) result: %v", err)
	}
	if err := checkWritable(context.Canceled, []evals.Result{ok}); err == nil {
		t.Error("interrupted run must not be written")
	}
	if err := checkWritable(nil, []evals.Result{infra, infra}); err == nil {
		t.Error("all-infrastructure-failure run must not be written")
	}
}
