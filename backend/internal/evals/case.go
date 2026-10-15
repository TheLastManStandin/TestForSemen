// Package evals measures agent quality on scripted cases against a real LLM.
package evals

import (
	"errors"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// Case is one user message with the expected outcome.
type Case struct {
	Name    string `yaml:"name"`
	Group   string `yaml:"group"`
	History []Turn `yaml:"history"` // earlier text-only turns of the conversation
	Input   string `yaml:"input"`
	Expect  Expect `yaml:"expect"`
}

// Turn is a prior message (role "user" or "assistant").
type Turn struct {
	Role    string `yaml:"role"`
	Content string `yaml:"content"`
}

// Expect describes the outcome. Listing no mutations means none are allowed.
type Expect struct {
	Created          []ExpenseMatch `yaml:"created"`
	Updated          []UpdateMatch  `yaml:"updated"`
	Deleted          []string       `yaml:"deleted"` // seed keys
	Ask              bool           `yaml:"ask"`     // reply must be a clarifying question
	ReplyContains    []string       `yaml:"reply_contains"`
	ReplyNotContains []string       `yaml:"reply_not_contains"`
	Tools            []string       `yaml:"tools"` // must be called at least once
}

// ExpenseMatch matches a created expense. Empty fields are not checked (Amount is required).
type ExpenseMatch struct {
	Amount   string `yaml:"amount"`
	Category string `yaml:"category"`
	Date     string `yaml:"date"`    // "today", "today-N" or YYYY-MM-DD
	Comment  string `yaml:"comment"` // case-insensitive substring
}

// UpdateMatch matches the new state of a seeded expense; fields left unset must stay unchanged.
type UpdateMatch struct {
	Key      string `yaml:"key"`
	Amount   string `yaml:"amount"`
	Category string `yaml:"category"`
	Date     string `yaml:"date"`
	Comment  string `yaml:"comment"` // substring; unset = comment must stay unchanged
}

// LoadCases reads and structurally validates a cases file.
func LoadCases(path string) ([]Case, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cases []Case
	if err := yaml.Unmarshal(data, &cases); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	seen := map[string]bool{}
	var errs []error
	for i, c := range cases {
		switch {
		case c.Name == "":
			errs = append(errs, fmt.Errorf("case #%d: empty name", i+1))
		case seen[c.Name]:
			errs = append(errs, fmt.Errorf("case %q: duplicate name", c.Name))
		case c.Group == "" || strings.TrimSpace(c.Input) == "":
			errs = append(errs, fmt.Errorf("case %q: group and input are required", c.Name))
		}
		seen[c.Name] = true
	}
	return cases, errors.Join(errs...)
}

// Filter keeps cases whose name or group contains substr (all if empty).
func Filter(cases []Case, substr string) []Case {
	if substr == "" {
		return cases
	}
	var out []Case
	for _, c := range cases {
		if strings.Contains(c.Name, substr) || strings.Contains(c.Group, substr) {
			out = append(out, c)
		}
	}
	return out
}
