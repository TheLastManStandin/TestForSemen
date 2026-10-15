package bot

import (
	"context"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

// MemberSet is an in-memory MemberChecker built from the configured family members.
// It follows the current FAMILY_MEMBERS, not the members table, which keeps removed
// members only for expense history.
type MemberSet map[int64]struct{}

func NewMemberSet(members []domain.Member) MemberSet {
	s := make(MemberSet, len(members))
	for _, m := range members {
		s[m.TelegramID] = struct{}{}
	}
	return s
}

func (s MemberSet) MemberExists(_ context.Context, telegramID int64) (bool, error) {
	_, ok := s[telegramID]
	return ok, nil
}
