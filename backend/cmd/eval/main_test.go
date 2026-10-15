package main

import (
	"context"
	"testing"

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
