package bot_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/bot"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

func TestMemberSet(t *testing.T) {
	set := bot.NewMemberSet([]domain.Member{{TelegramID: 111, Name: "Папа"}, {TelegramID: 222, Name: "Мама"}})

	ok, err := set.MemberExists(context.Background(), 111)
	require.NoError(t, err)
	assert.True(t, ok)

	// An id that exists only in the DB (removed from FAMILY_MEMBERS) is not allowed.
	ok, err = set.MemberExists(context.Background(), 333)
	require.NoError(t, err)
	assert.False(t, ok)
}
