package evals

import "github.com/TheLastManStandin/TestForSemen/backend/internal/domain"

// Change is an expense whose fields differ before and after the run.
type Change struct {
	Before, After domain.Expense
}

// Diff is the repository delta caused by one agent run.
type Diff struct {
	Created []domain.Expense
	Updated []Change
	Deleted []domain.Expense
}

// Empty reports whether nothing changed.
func (d Diff) Empty() bool { return len(d.Created)+len(d.Updated)+len(d.Deleted) == 0 }

// ComputeDiff compares two snapshots by expense ID.
func ComputeDiff(before, after []domain.Expense) Diff {
	prev := make(map[int64]domain.Expense, len(before))
	for _, e := range before {
		prev[e.ID] = e
	}
	var d Diff
	seen := make(map[int64]bool, len(after))
	for _, e := range after {
		seen[e.ID] = true
		old, ok := prev[e.ID]
		switch {
		case !ok:
			d.Created = append(d.Created, e)
		case old.Amount != e.Amount || old.Category != e.Category || !old.SpentAt.Equal(e.SpentAt) || old.Comment != e.Comment || old.UserID != e.UserID:
			d.Updated = append(d.Updated, Change{Before: old, After: e})
		}
	}
	for _, e := range before {
		if !seen[e.ID] {
			d.Deleted = append(d.Deleted, e)
		}
	}
	return d
}
