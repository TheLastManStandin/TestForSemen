package postgres

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/store/postgres/db"
)

// AppendChat stores agent conversation messages (chat-completions JSON) in order.
func (s *Store) AppendChat(ctx context.Context, userID int64, msgs []json.RawMessage) error {
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		q := s.q.WithTx(tx)
		for _, m := range msgs {
			if err := q.InsertChatMessage(ctx, db.InsertChatMessageParams{UserID: userID, Content: m}); err != nil {
				return fmt.Errorf("insert chat message: %w", err)
			}
		}
		return nil
	})
}

// RecentChat returns up to limit latest messages newer than since, oldest first.
func (s *Store) RecentChat(ctx context.Context, userID int64, since time.Time, limit int) ([]json.RawMessage, error) {
	rows, err := s.q.RecentChatMessages(ctx, db.RecentChatMessagesParams{UserID: userID, Since: since, Lim: clampInt32(limit)})
	if err != nil {
		return nil, fmt.Errorf("recent chat: %w", err)
	}
	return rows, nil
}

// SaveTrace stores one agent run.
func (s *Store) SaveTrace(ctx context.Context, t domain.Trace) error {
	steps := t.Steps
	if steps == nil {
		steps = []domain.TraceStep{}
	}
	stepsJSON, err := json.Marshal(steps)
	if err != nil {
		return fmt.Errorf("marshal steps: %w", err)
	}
	var stt *int32
	if t.STTms != nil {
		v := int32(*t.STTms)
		stt = &v
	}
	err = s.q.InsertTrace(ctx, db.InsertTraceParams{
		UserID: t.UserID, Source: t.Source, Input: t.Input, Output: nullIfEmpty(t.Output), Steps: stepsJSON,
		Model: t.Model, PromptTokens: int32(t.PromptTokens), CompletionTokens: int32(t.CompletionTokens),
		SttMs: stt, LatencyMs: int32(t.LatencyMs), Error: nullIfEmpty(t.Error),
	})
	if err != nil {
		return fmt.Errorf("insert trace: %w", err)
	}
	return nil
}

// ListTraces returns traces newest first and the total count.
func (s *Store) ListTraces(ctx context.Context, userID *int64, limit, offset int) ([]domain.Trace, int, error) {
	rows, err := s.q.ListTraces(ctx, db.ListTracesParams{UserID: userID, Off: clampInt32(offset), Lim: clampInt32(limit)})
	if err != nil {
		return nil, 0, fmt.Errorf("list traces: %w", err)
	}
	total, err := s.q.CountTraces(ctx, userID)
	if err != nil {
		return nil, 0, fmt.Errorf("count traces: %w", err)
	}
	out := make([]domain.Trace, 0, len(rows))
	for _, r := range rows {
		var steps []domain.TraceStep
		if err := json.Unmarshal(r.Steps, &steps); err != nil {
			return nil, 0, fmt.Errorf("unmarshal steps of trace %d: %w", r.ID, err)
		}
		var stt *int
		if r.SttMs != nil {
			v := int(*r.SttMs)
			stt = &v
		}
		out = append(out, domain.Trace{
			ID: r.ID, UserID: r.UserID, MemberName: r.MemberName, Source: r.Source, Input: r.Input,
			Output: deref(r.Output), Steps: steps, Model: r.Model,
			PromptTokens: int(r.PromptTokens), CompletionTokens: int(r.CompletionTokens),
			STTms: stt, LatencyMs: int(r.LatencyMs), Error: deref(r.Error), CreatedAt: r.CreatedAt,
		})
	}
	return out, int(total), nil
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func deref(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}
