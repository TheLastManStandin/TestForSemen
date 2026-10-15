package bot

import (
	"testing"

	"github.com/go-telegram/bot/models"
	"github.com/stretchr/testify/assert"
)

func TestToIncoming(t *testing.T) {
	text := &models.Update{Message: &models.Message{Chat: models.Chat{ID: 10}, From: &models.User{ID: 111}, Text: "кофе 350"}}
	in, ok := toIncoming(text)
	assert.True(t, ok)
	assert.Equal(t, Incoming{ChatID: 10, UserID: 111, Text: "кофе 350"}, in)

	voice := &models.Update{Message: &models.Message{Chat: models.Chat{ID: 10}, From: &models.User{ID: 111}, Voice: &models.Voice{FileID: "f1", Duration: 4}}}
	in, ok = toIncoming(voice)
	assert.True(t, ok)
	assert.Equal(t, &Voice{FileID: "f1", Duration: 4}, in.Voice)

	_, ok = toIncoming(&models.Update{})
	assert.False(t, ok, "updates without a message are ignored")
	_, ok = toIncoming(&models.Update{Message: &models.Message{Chat: models.Chat{ID: 10}}})
	assert.False(t, ok, "messages without a sender are ignored")
}
