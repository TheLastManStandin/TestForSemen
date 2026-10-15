-- +goose Up
CREATE TABLE members (
  telegram_id BIGINT PRIMARY KEY,
  name        TEXT NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE expenses (
  id          BIGSERIAL PRIMARY KEY,
  user_id     BIGINT NOT NULL REFERENCES members(telegram_id),
  amount      BIGINT NOT NULL CHECK (amount > 0),
  category    TEXT   NOT NULL CHECK (category IN
                ('food','transport','housing','entertainment','health','shopping','other')),
  spent_at    DATE   NOT NULL,
  comment     TEXT   NOT NULL DEFAULT '',
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX expenses_user_spent_idx ON expenses (user_id, spent_at DESC);
CREATE INDEX expenses_spent_idx      ON expenses (spent_at DESC);

CREATE TABLE chat_messages (
  id          BIGSERIAL PRIMARY KEY,
  user_id     BIGINT NOT NULL REFERENCES members(telegram_id),
  content     JSONB  NOT NULL,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX chat_messages_user_idx ON chat_messages (user_id, created_at DESC);

CREATE TABLE agent_traces (
  id                BIGSERIAL PRIMARY KEY,
  user_id           BIGINT NOT NULL REFERENCES members(telegram_id),
  source            TEXT   NOT NULL CHECK (source IN ('text','voice')),
  input             TEXT   NOT NULL,
  output            TEXT,
  steps             JSONB  NOT NULL,
  model             TEXT   NOT NULL,
  prompt_tokens     INT    NOT NULL DEFAULT 0,
  completion_tokens INT    NOT NULL DEFAULT 0,
  stt_ms            INT,
  latency_ms        INT    NOT NULL,
  error             TEXT,
  created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX agent_traces_created_idx ON agent_traces (created_at DESC);

-- +goose Down
DROP TABLE agent_traces;
DROP TABLE chat_messages;
DROP TABLE expenses;
DROP TABLE members;
