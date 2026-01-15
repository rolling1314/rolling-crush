-- name: CreateToolCall :one
INSERT INTO tool_calls (
    id,
    session_id,
    message_id,
    name,
    input,
    status,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, EXTRACT(EPOCH FROM NOW()) * 1000, EXTRACT(EPOCH FROM NOW()) * 1000
)
RETURNING *;

-- name: GetToolCall :one
SELECT * FROM tool_calls
WHERE id = $1 LIMIT 1;

-- name: ListToolCallsBySession :many
SELECT * FROM tool_calls
WHERE session_id = $1
ORDER BY created_at ASC;

-- name: ListToolCallsByMessage :many
SELECT * FROM tool_calls
WHERE message_id = $1
ORDER BY created_at ASC;

-- name: ListPendingToolCalls :many
SELECT * FROM tool_calls
WHERE session_id = $1 AND status IN ('pending', 'running')
ORDER BY created_at ASC;

-- name: UpdateToolCallStatus :exec
UPDATE tool_calls
SET
    status = $1,
    started_at = CASE WHEN $1 = 'running' AND started_at IS NULL THEN EXTRACT(EPOCH FROM NOW()) * 1000 ELSE started_at END,
    updated_at = EXTRACT(EPOCH FROM NOW()) * 1000
WHERE id = $2;

-- name: UpdateToolCallInput :exec
UPDATE tool_calls
SET
    input = $1,
    status = CASE WHEN status = 'pending' THEN 'running' ELSE status END,
    started_at = CASE WHEN started_at IS NULL THEN EXTRACT(EPOCH FROM NOW()) * 1000 ELSE started_at END,
    updated_at = EXTRACT(EPOCH FROM NOW()) * 1000
WHERE id = $2;

-- name: UpdateToolCallResult :exec
UPDATE tool_calls
SET
    result = $1,
    is_error = $2,
    error_message = $3,
    status = CASE WHEN $2 THEN 'error' ELSE 'completed' END,
    finished_at = EXTRACT(EPOCH FROM NOW()) * 1000,
    updated_at = EXTRACT(EPOCH FROM NOW()) * 1000
WHERE id = $4;

-- name: CancelToolCall :exec
UPDATE tool_calls
SET
    status = 'cancelled',
    finished_at = EXTRACT(EPOCH FROM NOW()) * 1000,
    updated_at = EXTRACT(EPOCH FROM NOW()) * 1000
WHERE id = $1 AND status IN ('pending', 'running');

-- name: CancelSessionToolCalls :exec
UPDATE tool_calls
SET
    status = 'cancelled',
    finished_at = EXTRACT(EPOCH FROM NOW()) * 1000,
    updated_at = EXTRACT(EPOCH FROM NOW()) * 1000
WHERE session_id = $1 AND status IN ('pending', 'running');

-- name: DeleteToolCall :exec
DELETE FROM tool_calls
WHERE id = $1;

-- name: DeleteSessionToolCalls :exec
DELETE FROM tool_calls
WHERE session_id = $1;
