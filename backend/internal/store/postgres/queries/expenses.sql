-- name: InsertExpense :one
WITH ins AS (
  INSERT INTO expenses (user_id, amount, category, spent_at, comment)
  VALUES ($1, $2, $3, $4, $5)
  RETURNING *
)
SELECT ins.id, ins.user_id, m.name AS member_name, ins.amount, ins.category, ins.spent_at,
       ins.comment, ins.created_at, ins.updated_at, ins.version
FROM ins JOIN members m ON m.telegram_id = ins.user_id;

-- name: GetExpense :one
SELECT e.id, e.user_id, m.name AS member_name, e.amount, e.category, e.spent_at,
       e.comment, e.created_at, e.updated_at, e.version
FROM expenses e JOIN members m ON m.telegram_id = e.user_id
WHERE e.id = sqlc.arg(id)
  AND (sqlc.narg(owner_id)::bigint IS NULL OR e.user_id = sqlc.narg(owner_id));

-- name: ListExpenses :many
SELECT e.id, e.user_id, m.name AS member_name, e.amount, e.category, e.spent_at,
       e.comment, e.created_at, e.updated_at, e.version
FROM expenses e JOIN members m ON m.telegram_id = e.user_id
WHERE (sqlc.narg(user_id)::bigint IS NULL OR e.user_id = sqlc.narg(user_id))
  AND (sqlc.narg(date_from)::date IS NULL OR e.spent_at >= sqlc.narg(date_from))
  AND (sqlc.narg(date_to)::date   IS NULL OR e.spent_at <= sqlc.narg(date_to))
  AND (sqlc.narg(category)::text  IS NULL OR e.category = sqlc.narg(category))
  AND (sqlc.narg(text)::text      IS NULL OR e.comment ILIKE '%' || sqlc.narg(text) || '%')
ORDER BY
  CASE WHEN sqlc.arg(sort_recent)::bool THEN e.created_at END DESC,
  e.spent_at DESC, e.id DESC
LIMIT sqlc.arg(lim) OFFSET sqlc.arg(off);

-- name: SummarizeExpenses :one
SELECT count(*)::bigint AS cnt, coalesce(sum(e.amount), 0)::bigint AS total
FROM expenses e
WHERE (sqlc.narg(user_id)::bigint IS NULL OR e.user_id = sqlc.narg(user_id))
  AND (sqlc.narg(date_from)::date IS NULL OR e.spent_at >= sqlc.narg(date_from))
  AND (sqlc.narg(date_to)::date   IS NULL OR e.spent_at <= sqlc.narg(date_to))
  AND (sqlc.narg(category)::text  IS NULL OR e.category = sqlc.narg(category))
  AND (sqlc.narg(text)::text      IS NULL OR e.comment ILIKE '%' || sqlc.narg(text) || '%');

-- name: UpdateExpense :one
-- Partial update: NULL args keep the current value. Owner and expected version are checked
-- against the current row in the same statement, so concurrent edits are never lost silently.
WITH upd AS (
  UPDATE expenses SET
    user_id  = COALESCE(sqlc.narg(user_id)::bigint, user_id),
    amount   = COALESCE(sqlc.narg(amount)::bigint, amount),
    category = COALESCE(sqlc.narg(category)::text, category),
    spent_at = COALESCE(sqlc.narg(spent_at)::date, spent_at),
    comment  = COALESCE(sqlc.narg(comment)::text, comment),
    version  = version + 1,
    updated_at = now()
  WHERE expenses.id = sqlc.arg(id)
    AND (sqlc.narg(owner_id)::bigint IS NULL OR expenses.user_id = sqlc.narg(owner_id))
    AND (sqlc.narg(expected_version)::bigint IS NULL OR expenses.version = sqlc.narg(expected_version))
  RETURNING *
)
SELECT upd.id, upd.user_id, m.name AS member_name, upd.amount, upd.category, upd.spent_at,
       upd.comment, upd.created_at, upd.updated_at, upd.version
FROM upd JOIN members m ON m.telegram_id = upd.user_id;

-- name: DeleteExpense :one
-- Returns the row as it was deleted, so a concurrent edit is never reported as the old version.
WITH del AS (
  DELETE FROM expenses
  WHERE expenses.id = sqlc.arg(id)
    AND (sqlc.narg(owner_id)::bigint IS NULL OR expenses.user_id = sqlc.narg(owner_id))
  RETURNING *
)
SELECT del.id, del.user_id, m.name AS member_name, del.amount, del.category, del.spent_at,
       del.comment, del.created_at, del.updated_at, del.version
FROM del JOIN members m ON m.telegram_id = del.user_id;

-- name: AggregateExpenses :many
SELECT e.spent_at, e.user_id, m.name AS member_name, e.category,
       sum(e.amount)::bigint AS total, count(*)::int AS cnt
FROM expenses e JOIN members m ON m.telegram_id = e.user_id
WHERE e.spent_at >= sqlc.arg(date_from) AND e.spent_at <= sqlc.arg(date_to)
  AND (sqlc.narg(user_id)::bigint IS NULL OR e.user_id = sqlc.narg(user_id))
  AND (sqlc.narg(category)::text  IS NULL OR e.category = sqlc.narg(category))
GROUP BY e.spent_at, e.user_id, m.name, e.category
ORDER BY e.spent_at, e.user_id, e.category;
