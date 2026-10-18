package expense

import (
	"context"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/events"
)

// MaxAmount is 100 million roubles.
const MaxAmount domain.Money = 10_000_000_000

// Limits.
const (
	MaxComment   = 500
	DefaultLimit = 50
	MaxLimit     = 200
)

// Publisher receives events after successful mutations.
type Publisher interface {
	Publish(events.Event)
}

type noopPublisher struct{}

func (noopPublisher) Publish(events.Event) {}

// Scope limits which expenses an operation may see or touch.
type Scope struct {
	UserID *int64 // nil: all members (web panel); set: only this member (bot)
}

// All is the panel scope.
func All() Scope { return Scope{} }

// Own is a single member's scope.
func Own(userID int64) Scope { return Scope{UserID: &userID} }

// Service is the single home of expense business rules, shared by REST and the agent.
type Service struct {
	repo Repository
	pub  Publisher
	loc  *time.Location
	now  func() time.Time
}

// NewService wires a service. pub, loc and now may be nil (no-op, UTC, time.Now).
func NewService(repo Repository, pub Publisher, loc *time.Location, now func() time.Time) *Service {
	if pub == nil {
		pub = noopPublisher{}
	}
	if loc == nil {
		loc = time.UTC
	}
	if now == nil {
		now = time.Now
	}
	return &Service{repo: repo, pub: pub, loc: loc, now: now}
}

// Now returns the current time in the family's location.
func (s *Service) Now() time.Time { return s.now().In(s.loc) }

// Today returns today's domain date in the family's location.
func (s *Service) Today() time.Time { return domain.DateOf(s.Now()) }

func (s *Service) SyncMembers(ctx context.Context, members []domain.Member) error {
	return s.repo.UpsertMembers(ctx, members)
}

func (s *Service) ListMembers(ctx context.Context) ([]domain.Member, error) {
	return s.repo.ListMembers(ctx)
}

func (s *Service) MemberExists(ctx context.Context, telegramID int64) (bool, error) {
	return s.repo.MemberExists(ctx, telegramID)
}

// CreateInput is a new expense. Zero SpentAt means today.
type CreateInput struct {
	UserID   int64
	Amount   domain.Money
	Category domain.Category
	SpentAt  time.Time
	Comment  string
}

func (s *Service) Create(ctx context.Context, in CreateInput) (domain.Expense, error) {
	e := domain.Expense{
		UserID:   in.UserID,
		Amount:   in.Amount,
		Category: in.Category,
		SpentAt:  domain.DateOf(in.SpentAt),
		Comment:  strings.TrimSpace(in.Comment),
	}
	if in.SpentAt.IsZero() {
		e.SpentAt = s.Today()
	}
	if err := s.validate(ctx, e); err != nil {
		return domain.Expense{}, err
	}
	created, err := s.repo.Insert(ctx, e)
	if err != nil {
		return domain.Expense{}, err
	}
	s.pub.Publish(events.Event{Type: events.ExpenseCreated, Expense: created})
	return created, nil
}

func (s *Service) Get(ctx context.Context, scope Scope, id int64) (domain.Expense, error) {
	return s.repo.Get(ctx, id, scope.UserID)
}

// Page is one page of a listing plus totals over the whole filter.
type Page struct {
	Items []domain.Expense
	Total int
	Sum   domain.Money
}

func (s *Service) List(ctx context.Context, scope Scope, q ListQuery) (Page, error) {
	if scope.UserID != nil {
		q.UserID = scope.UserID
	}
	if q.Limit <= 0 {
		q.Limit = DefaultLimit
	}
	q.Limit = min(q.Limit, MaxLimit)
	q.Offset = max(q.Offset, 0)
	if q.From != nil && q.To != nil && q.To.Before(*q.From) {
		return Page{}, domain.Invalid("to", "конец периода раньше начала")
	}
	items, err := s.repo.List(ctx, q)
	if err != nil {
		return Page{}, err
	}
	total, sum, err := s.repo.Count(ctx, q)
	if err != nil {
		return Page{}, err
	}
	return Page{Items: items, Total: total, Sum: sum}, nil
}

// Patch holds optional field updates. Comment "" clears the comment.
// Only the provided fields are written, so concurrent edits of other fields survive.
type Patch struct {
	UserID   *int64
	Amount   *domain.Money
	Category *domain.Category
	SpentAt  *time.Time
	Comment  *string
	// Version, if set, is the version the caller last saw (optimistic concurrency, web panel).
	// A mismatch with the stored version yields domain.ErrConflict and nothing is written.
	// The bot sends no version: its partial updates simply apply on top of the current row.
	Version *int64
}

func (s *Service) Update(ctx context.Context, scope Scope, id int64, p Patch) (domain.Expense, error) {
	if p.UserID == nil && p.Amount == nil && p.Category == nil && p.SpentAt == nil && p.Comment == nil {
		// Nothing to change: do not bump the version or publish an event.
		return domain.Expense{}, domain.Invalid("fields", "не передано ни одного поля для изменения")
	}
	if p.SpentAt != nil {
		p.SpentAt = new(domain.DateOf(*p.SpentAt))
	}
	if p.Comment != nil {
		p.Comment = new(strings.TrimSpace(*p.Comment))
	}
	// Validate the merged result. The read is only for validation (and a fast conflict answer):
	// the repository write re-checks owner and version atomically and touches only patched fields.
	cur, err := s.repo.Get(ctx, id, scope.UserID)
	if err != nil {
		return domain.Expense{}, err
	}
	if p.UserID != nil && scope.UserID != nil && *p.UserID != *scope.UserID {
		return domain.Expense{}, domain.Invalid("user_id", "нельзя передать трату другому члену семьи")
	}
	if p.Version != nil && *p.Version != cur.Version {
		return domain.Expense{}, domain.ErrConflict
	}
	if err := s.validate(ctx, apply(cur, p)); err != nil {
		return domain.Expense{}, err
	}
	updated, err := s.repo.Update(ctx, id, p, scope.UserID)
	if err != nil {
		return domain.Expense{}, err
	}
	s.pub.Publish(events.Event{Type: events.ExpenseUpdated, Expense: updated})
	return updated, nil
}

// apply returns e with the non-nil fields of p set.
func apply(e domain.Expense, p Patch) domain.Expense {
	if p.UserID != nil {
		e.UserID = *p.UserID
	}
	if p.Amount != nil {
		e.Amount = *p.Amount
	}
	if p.Category != nil {
		e.Category = *p.Category
	}
	if p.SpentAt != nil {
		e.SpentAt = *p.SpentAt
	}
	if p.Comment != nil {
		e.Comment = *p.Comment
	}
	return e
}

// Delete removes an expense and returns the version that was actually deleted.
func (s *Service) Delete(ctx context.Context, scope Scope, id int64) (domain.Expense, error) {
	e, err := s.repo.Delete(ctx, id, scope.UserID)
	if err != nil {
		return domain.Expense{}, err
	}
	s.pub.Publish(events.Event{Type: events.ExpenseDeleted, Expense: e})
	return e, nil
}

func (s *Service) validate(ctx context.Context, e domain.Expense) error {
	switch {
	case e.Amount <= 0:
		return domain.Invalid("amount", "сумма должна быть больше нуля")
	case e.Amount > MaxAmount:
		return domain.Invalid("amount", "сумма слишком большая")
	case !e.Category.Valid():
		return domain.Invalid("category", fmt.Sprintf("неизвестная категория %q", e.Category))
	case e.SpentAt.After(s.Today().AddDate(0, 0, 1)):
		return domain.Invalid("spent_at", "дата траты не может быть в будущем")
	case e.SpentAt.Year() < 2000:
		return domain.Invalid("spent_at", "слишком старая дата")
	case utf8.RuneCountInString(e.Comment) > MaxComment:
		return domain.Invalid("comment", fmt.Sprintf("комментарий длиннее %d символов", MaxComment))
	}
	ok, err := s.repo.MemberExists(ctx, e.UserID)
	if err != nil {
		return fmt.Errorf("check member: %w", err)
	}
	if !ok {
		return domain.Invalid("user_id", "неизвестный член семьи")
	}
	return nil
}
