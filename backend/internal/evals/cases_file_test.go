package evals

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/TheLastManStandin/TestForSemen/backend/internal/agent"
	"github.com/TheLastManStandin/TestForSemen/backend/internal/domain"
)

// TestCasesFileIsValid catches typos in cases.yaml without calling an LLM.
func TestCasesFileIsValid(t *testing.T) {
	cases, err := LoadCases("../../evals/cases.yaml")
	require.NoError(t, err)
	assert.Len(t, cases, 45)

	keys := SeedKeys()
	groups := map[string]int{}
	for _, c := range cases {
		groups[c.Group]++
		for _, turn := range c.History {
			assert.Contains(t, []string{"user", "assistant"}, turn.Role, c.Name)
		}
		for _, m := range c.Expect.Created {
			_, err := domain.ParseMoney(m.Amount)
			assert.NoError(t, err, "%s: amount", c.Name)
			assert.True(t, m.Category == "" || domain.Category(m.Category).Valid(), "%s: category %q", c.Name, m.Category)
			if m.Date != "" {
				_, err := ResolveDate(m.Date, today)
				assert.NoError(t, err, "%s: date", c.Name)
			}
		}
		for _, u := range c.Expect.Updated {
			assert.True(t, slices.Contains(keys, u.Key), "%s: unknown key %q", c.Name, u.Key)
			assert.True(t, u.Category == "" || domain.Category(u.Category).Valid(), "%s: category", c.Name)
		}
		for _, key := range c.Expect.Deleted {
			assert.True(t, slices.Contains(keys, key), "%s: unknown key %q", c.Name, key)
		}
		for _, tool := range c.Expect.Tools {
			assert.True(t, slices.Contains(agent.ToolNames(), tool), "%s: unknown tool %q", c.Name, tool)
		}
	}
	for _, g := range []string{"add", "query", "update", "delete", "security", "offtopic", "robustness"} {
		assert.Positive(t, groups[g], "group %s has cases", g)
	}
}
