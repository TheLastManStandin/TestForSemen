-- name: InsertChatMessage :exec
INSERT INTO chat_messages (user_id, content) VALUES ($1, $2);

-- name: RecentChatMessages :many
SELECT content FROM (
  SELECT id, content FROM chat_messages
  WHERE user_id = $1 AND created_at >= sqlc.arg(since)
  ORDER BY id DESC
  LIMIT sqlc.arg(lim)
) recent ORDER BY id;

-- name: InsertTrace :exec
INSERT INTO agent_traces (user_id, source, input, output, steps, model,
  prompt_tokens, completion_tokens, stt_ms, latency_ms, error)
VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11);

-- name: ListTraces :many
SELECT t.id, t.user_id, m.name AS member_name, t.source, t.input, t.output, t.steps, t.model,
       t.prompt_tokens, t.completion_tokens, t.stt_ms, t.latency_ms, t.error, t.created_at
FROM agent_traces t JOIN members m ON m.telegram_id = t.user_id
WHERE (sqlc.narg(user_id)::bigint IS NULL OR t.user_id = sqlc.narg(user_id))
ORDER BY t.id DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: CountTraces :one
SELECT count(*)::bigint FROM agent_traces
WHERE (sqlc.narg(user_id)::bigint IS NULL OR user_id = sqlc.narg(user_id));
