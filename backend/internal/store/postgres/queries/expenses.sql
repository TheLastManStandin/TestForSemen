-- name: InsertExpense :one
WITH ins AS (
  INSERT INTO expenses (user_id, amount, category, spent_at, comment)
  VALUES ($1, $2, $3, $4, $5)
  RETURNING *
)
SELECT ins.id, ins.user_id, m.name AS member_name, ins.amount, ins.category, ins.spent_at,
       ins.comment, ins.created_at, ins.updated_at
FROM ins JOIN members m ON m.telegram_id = ins.user_id;

-- name: GetExpense :one
SELECT e.id, e.user_id, m.name AS member_name, e.amount, e.category, e.spent_at,
       e.comment, e.created_at, e.updated_at
FROM expenses e JOIN members m ON m.telegram_id = e.user_id
WHERE e.id = sqlc.arg(id)
  AND (sqlc.narg(owner_id)::bigint IS NULL OR e.user_id = sqlc.narg(owner_id));

-- name: ListExpenses :many
SELECT e.id, e.user_id, m.name AS member_name, e.amount, e.category, e.spent_at,
       e.comment, e.created_at, e.updated_at
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
WITH upd AS (
  UPDATE expenses SET
    user_id = sqlc.arg(user_id), amount = sqlc.arg(amount), category = sqlc.arg(category),
    spent_at = sqlc.arg(spent_at), comment = sqlc.arg(comment), updated_at = now()
  WHERE expenses.id = sqlc.arg(id)
    AND (sqlc.narg(owner_id)::bigint IS NULL OR expenses.user_id = sqlc.narg(owner_id))
  RETURNING *
)
SELECT upd.id, upd.user_id, m.name AS member_name, upd.amount, upd.category, upd.spent_at,
       upd.comment, upd.created_at, upd.updated_at
FROM upd JOIN members m ON m.telegram_id = upd.user_id;

-- name: DeleteExpense :execrows
DELETE FROM expenses
WHERE id = sqlc.arg(id)
  AND (sqlc.narg(owner_id)::bigint IS NULL OR user_id = sqlc.narg(owner_id));

-- name: AggregateExpenses :many
SELECT e.spent_at, e.user_id, m.name AS member_name, e.category,
       sum(e.amount)::bigint AS total, count(*)::int AS cnt
FROM expenses e JOIN members m ON m.telegram_id = e.user_id
WHERE e.spent_at >= sqlc.arg(date_from) AND e.spent_at <= sqlc.arg(date_to)
  AND (sqlc.narg(user_id)::bigint IS NULL OR e.user_id = sqlc.narg(user_id))
  AND (sqlc.narg(category)::text  IS NULL OR e.category = sqlc.narg(category))
GROUP BY e.spent_at, e.user_id, m.name, e.category
ORDER BY e.spent_at, e.user_id, e.category;
