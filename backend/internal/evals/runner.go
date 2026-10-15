package evals

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/agent"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense/memrepo"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/llm"
)

// Runner executes cases, each in its own fresh world.
type Runner struct {
	LLM      llm.Client
	Model    string
	Parallel int
}

// Result is the outcome of one case.
type Result struct {
	Case     Case
	Failures []string
	Reply    string
	Tools    []string
	Latency  time.Duration
	Usage    llm.Usage
	Err      string // infrastructure error (LLM unreachable, etc.)
}

// Passed reports whether the case passed.
func (r Result) Passed() bool { return r.Err == "" && len(r.Failures) == 0 }

// Run executes cases with bounded parallelism; results keep the input order.
func (r *Runner) Run(ctx context.Context, cases []Case) []Result {
	results := make([]Result, len(cases))
	sem := make(chan struct{}, max(r.Parallel, 1))
	var wg sync.WaitGroup
	for i, c := range cases {
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			results[i] = r.runCase(ctx, c)
		}()
	}
	wg.Wait()
	return results
}

func (r *Runner) runCase(ctx context.Context, c Case) Result {
	res := Result{Case: c}
	clock := func() time.Time { return Now }

	repo := memrepo.New()
	if err := repo.UpsertMembers(ctx, Members); err != nil {
		res.Err = err.Error()
		return res
	}
	svc := expense.NewService(repo, nil, Now.Location(), clock)
	seeded, err := Seed(ctx, svc)
	if err != nil {
		res.Err = err.Error()
		return res
	}

	mem := agent.NewMemoryStore(clock)
	if err := loadHistory(ctx, mem, c.History); err != nil {
		res.Err = err.Error()
		return res
	}
	ag := agent.New(r.LLM, svc, mem, mem, agent.Config{Model: r.Model, Temperature: 0.1}, slog.New(slog.DiscardHandler))

	before := repo.Snapshot()
	start := time.Now()
	out, err := ag.Run(ctx, agent.Input{UserID: User.TelegramID, Text: c.Input, Source: agent.SourceText})
	res.Latency = time.Since(start)
	res.Usage = out.Usage
	if err != nil {
		res.Err = err.Error()
		return res
	}
	res.Reply = out.Reply
	for _, s := range out.Steps {
		res.Tools = append(res.Tools, s.Tool)
	}
	diff := ComputeDiff(before, repo.Snapshot())
	res.Failures = Check(c.Expect, seeded, Outcome{Diff: diff, Reply: out.Reply, Tools: res.Tools}, domain.DateOf(Now))
	return res
}

func loadHistory(ctx context.Context, h agent.History, turns []Turn) error {
	if len(turns) == 0 {
		return nil
	}
	msgs := make([]json.RawMessage, 0, len(turns))
	for _, t := range turns {
		b, err := json.Marshal(llm.Message{Role: llm.Role(t.Role), Content: t.Content})
		if err != nil {
			return err
		}
		msgs = append(msgs, b)
	}
	return h.AppendChat(ctx, User.TelegramID, msgs)
}
