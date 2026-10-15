-- name: UpsertMember :exec
INSERT INTO members (telegram_id, name) VALUES ($1, $2)
ON CONFLICT (telegram_id) DO UPDATE SET name = EXCLUDED.name;

-- name: ListMembers :many
SELECT telegram_id, name FROM members ORDER BY name, telegram_id;

-- name: MemberExists :one
SELECT EXISTS (SELECT 1 FROM members WHERE telegram_id = $1);
