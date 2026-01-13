-- name: GetMessage :one
SELECT *
FROM messages
WHERE id = $1 LIMIT 1;

-- name: ListMessagesBySession :many
SELECT *
FROM messages
WHERE session_id = $1
ORDER BY created_at ASC;

-- name: CreateMessage :one
INSERT INTO messages (
    id,
    session_id,
    role,
    parts,
    model,
    provider,
    is_summary_message,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, EXTRACT(EPOCH FROM NOW()) * 1000, EXTRACT(EPOCH FROM NOW()) * 1000
)
RETURNING *;

-- name: UpdateMessage :exec
UPDATE messages
SET
    parts = $1,
    finished_at = $2,
    updated_at = EXTRACT(EPOCH FROM NOW()) * 1000
WHERE id = $3;


-- name: DeleteMessage :exec
DELETE FROM messages
WHERE id = $1;

-- name: DeleteSessionMessages :exec
DELETE FROM messages
WHERE session_id = $1;
