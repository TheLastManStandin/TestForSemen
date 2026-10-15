package postgres

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/store/postgres/db"
)

func (s *Store) UpsertMembers(ctx context.Context, members []domain.Member) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		for _, m := range members {
			if err := q.UpsertMember(ctx, db.UpsertMemberParams{TelegramID: m.TelegramID, Name: m.Name}); err != nil {
				return fmt.Errorf("upsert member %d: %w", m.TelegramID, err)
			}
		}
		return nil
	})
}

func (s *Store) ListMembers(ctx context.Context) ([]domain.Member, error) {
	rows, err := s.q.ListMembers(ctx)
	if err != nil {
		return nil, fmt.Errorf("list members: %w", err)
	}
	out := make([]domain.Member, 0, len(rows))
	for _, r := range rows {
		out = append(out, domain.Member{TelegramID: r.TelegramID, Name: r.Name})
	}
	return out, nil
}

func (s *Store) MemberExists(ctx context.Context, telegramID int64) (bool, error) {
	ok, err := s.q.MemberExists(ctx, telegramID)
	if err != nil {
		return false, fmt.Errorf("member exists: %w", err)
	}
	return ok, nil
}
