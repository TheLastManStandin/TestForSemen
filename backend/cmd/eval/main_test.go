package main

import "testing"

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
