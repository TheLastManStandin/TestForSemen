package agent_test

import (
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/agent"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

func TestBuildSystemPrompt(t *testing.T) {
	loc, err := time.LoadLocation("Europe/Moscow")
	require.NoError(t, err)
	p := agent.BuildSystemPrompt(time.Date(2026, 10, 15, 12, 0, 0, 0, loc))

	assert.Contains(t, p, "Сегодня: 2026-10-15, четверг")
	assert.Contains(t, p, "Europe/Moscow")
	assert.Contains(t, p, "- 2026-10-15 — четверг (сегодня)")
	assert.Contains(t, p, "- 2026-10-14 — среда (вчера)")
	assert.Contains(t, p, "- 2026-10-02 — пятница")
	assert.NotContains(t, p, "2026-10-01 —", "calendar covers exactly 14 days")
	assert.Equal(t, 14, strings.Count(p, " — понедельник")+strings.Count(p, " — вторник")+strings.Count(p, " — среда")+
		strings.Count(p, " — четверг")+strings.Count(p, " — пятница")+strings.Count(p, " — суббота")+strings.Count(p, " — воскресенье"))
	for _, c := range domain.Categories {
		assert.Contains(t, p, "- "+string(c)+" — "+c.RU())
	}
	for _, period := range domain.Periods {
		assert.Contains(t, p, string(period))
	}
	assert.Contains(t, p, "find_expenses")
	assert.Contains(t, p, "только свои траты")
}
