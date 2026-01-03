-- name: CreateSession :one
INSERT INTO sessions (
    id,
    parent_session_id,
    title,
    message_count,
    prompt_tokens,
    completion_tokens,
    cost,
    summary_message_id,
    updated_at,
    created_at
) VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    null,
    EXTRACT(EPOCH FROM NOW()) * 1000,
    EXTRACT(EPOCH FROM NOW()) * 1000
) RETURNING *;

-- name: GetSessionByID :one
SELECT *
FROM sessions
WHERE id = $1 LIMIT 1;

-- name: ListSessions :many
SELECT *
FROM sessions
WHERE parent_session_id is NULL
ORDER BY created_at DESC;

-- name: UpdateSession :one
UPDATE sessions
SET
    title = $1,
    prompt_tokens = $2,
    completion_tokens = $3,
    summary_message_id = $4,
    cost = $5
WHERE id = $6
RETURNING *;


-- name: DeleteSession :exec
DELETE FROM sessions
WHERE id = $1;
