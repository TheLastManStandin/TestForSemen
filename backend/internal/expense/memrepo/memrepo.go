// Package memrepo is an in-memory expense.Repository for unit tests and agent evals.
package memrepo

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/expense"
)

// Repo is safe for concurrent use.
type Repo struct {
	mu      sync.Mutex
	members map[int64]string
	items   map[int64]domain.Expense
	nextID  int64
	last    time.Time
}

var _ expense.Repository = (*Repo)(nil)

// New returns an empty repository.
func New() *Repo {
	return &Repo{members: map[int64]string{}, items: map[int64]domain.Expense{}}
}

// stamp returns a strictly increasing timestamp so ordering by creation time is deterministic.
func (r *Repo) stamp() time.Time {
	now := time.Now().UTC()
	if !now.After(r.last) {
		now = r.last.Add(time.Microsecond)
	}
	r.last = now
	return now
}

func (r *Repo) UpsertMembers(_ context.Context, members []domain.Member) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, m := range members {
		r.members[m.TelegramID] = m.Name
	}
	return nil
}

func (r *Repo) ListMembers(_ context.Context) ([]domain.Member, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Member, 0, len(r.members))
	for id, name := range r.members {
		out = append(out, domain.Member{TelegramID: id, Name: name})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].TelegramID < out[j].TelegramID
	})
	return out, nil
}

func (r *Repo) MemberExists(_ context.Context, id int64) (bool, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	_, ok := r.members[id]
	return ok, nil
}

func (r *Repo) Insert(_ context.Context, e domain.Expense) (domain.Expense, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	name, ok := r.members[e.UserID]
	if !ok {
		return domain.Expense{}, fmt.Errorf("member %d does not exist", e.UserID)
	}
	r.nextID++
	e.ID = r.nextID
	e.MemberName = name
	e.CreatedAt = r.stamp()
	e.UpdatedAt = e.CreatedAt
	r.items[e.ID] = e
	return e, nil
}

func (r *Repo) Get(_ context.Context, id int64, owner *int64) (domain.Expense, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e, ok := r.items[id]
	if !ok || (owner != nil && e.UserID != *owner) {
		return domain.Expense{}, domain.ErrNotFound
	}
	e.MemberName = r.members[e.UserID]
	return e, nil
}

func (r *Repo) List(_ context.Context, q expense.ListQuery) ([]domain.Expense, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := r.filter(q)
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if q.SortRecent && !a.CreatedAt.Equal(b.CreatedAt) {
			return a.CreatedAt.After(b.CreatedAt)
		}
		if !a.SpentAt.Equal(b.SpentAt) {
			return a.SpentAt.After(b.SpentAt)
		}
		return a.ID > b.ID
	})
	if q.Offset >= len(out) {
		return []domain.Expense{}, nil
	}
	out = out[q.Offset:]
	if q.Limit > 0 && len(out) > q.Limit {
		out = out[:q.Limit]
	}
	return out, nil
}

func (r *Repo) Count(_ context.Context, q expense.ListQuery) (int, domain.Money, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	var sum domain.Money
	matched := r.filter(q)
	for _, e := range matched {
		sum += e.Amount
	}
	return len(matched), sum, nil
}

func (r *Repo) Update(_ context.Context, e domain.Expense, owner *int64) (domain.Expense, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	cur, ok := r.items[e.ID]
	if !ok || (owner != nil && cur.UserID != *owner) {
		return domain.Expense{}, domain.ErrNotFound
	}
	name, ok := r.members[e.UserID]
	if !ok {
		return domain.Expense{}, fmt.Errorf("member %d does not exist", e.UserID)
	}
	e.CreatedAt = cur.CreatedAt
	e.UpdatedAt = r.stamp()
	e.MemberName = name
	r.items[e.ID] = e
	return e, nil
}

func (r *Repo) Delete(_ context.Context, id int64, owner *int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if cur, ok := r.items[id]; !ok || (owner != nil && cur.UserID != *owner) {
		return domain.ErrNotFound
	}
	delete(r.items, id)
	return nil
}

func (r *Repo) Aggregate(_ context.Context, q expense.AggQuery) ([]expense.AggRow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	type key struct {
		date time.Time
		user int64
		cat  domain.Category
	}
	groups := map[key]*expense.AggRow{}
	for _, e := range r.filter(expense.ListQuery{UserID: q.UserID, From: &q.From, To: &q.To, Category: q.Category}) {
		k := key{e.SpentAt, e.UserID, e.Category}
		g, ok := groups[k]
		if !ok {
			g = &expense.AggRow{Date: e.SpentAt, UserID: e.UserID, MemberName: e.MemberName, Category: e.Category}
			groups[k] = g
		}
		g.Total += e.Amount
		g.Count++
	}
	out := make([]expense.AggRow, 0, len(groups))
	for _, g := range groups {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if !a.Date.Equal(b.Date) {
			return a.Date.Before(b.Date)
		}
		if a.UserID != b.UserID {
			return a.UserID < b.UserID
		}
		return a.Category < b.Category
	})
	return out, nil
}

// Snapshot returns all expenses ordered by ID. Used by evals to diff state.
func (r *Repo) Snapshot() []domain.Expense {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]domain.Expense, 0, len(r.items))
	for _, e := range r.items {
		e.MemberName = r.members[e.UserID]
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// filter must be called with r.mu held.
func (r *Repo) filter(q expense.ListQuery) []domain.Expense {
	text := strings.ToLower(q.Text)
	var out []domain.Expense
	for _, e := range r.items {
		switch {
		case q.UserID != nil && e.UserID != *q.UserID,
			q.From != nil && e.SpentAt.Before(*q.From),
			q.To != nil && e.SpentAt.After(*q.To),
			q.Category != nil && e.Category != *q.Category,
			text != "" && !strings.Contains(strings.ToLower(e.Comment), text):
			continue
		}
		e.MemberName = r.members[e.UserID]
		out = append(out, e)
	}
	return out
}
