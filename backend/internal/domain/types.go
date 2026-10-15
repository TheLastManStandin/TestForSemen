package domain

import (
	"encoding/json"
	"time"
)

// Expense is a single spending record owned by a family member.
type Expense struct {
	ID         int64
	UserID     int64
	MemberName string
	Amount     Money
	Category   Category
	SpentAt    time.Time // domain date, see DateOf
	Comment    string    // empty means no comment
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// Member is a family member allowed to use the bot.
type Member struct {
	TelegramID int64
	Name       string
}

// Trace records one agent run for the panel's agent journal.
type Trace struct {
	ID               int64
	UserID           int64
	MemberName       string
	Source           string // "text" | "voice"
	Input            string
	Output           string // empty means none
	Steps            []TraceStep
	Model            string
	PromptTokens     int
	CompletionTokens int
	STTms            *int
	LatencyMs        int
	Error            string // empty means none
	CreatedAt        time.Time
}

// TraceStep is one tool call made by the agent.
type TraceStep struct {
	Tool   string          `json:"tool"`
	Args   json.RawMessage `json:"args"`
	Result json.RawMessage `json:"result"`
	Ms     int64           `json:"ms"`
}
