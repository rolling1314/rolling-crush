-- name: CreateSessionModelConfig :one
INSERT INTO session_model_configs (
    id,
    session_id,
    config_json,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3,
    EXTRACT(EPOCH FROM NOW()) * 1000,
    EXTRACT(EPOCH FROM NOW()) * 1000
)
RETURNING *;

-- name: GetSessionModelConfig :one
SELECT * FROM session_model_configs
WHERE session_id = $1
LIMIT 1;

-- name: UpdateSessionModelConfig :one
UPDATE session_model_configs
SET
    config_json = $2,
    updated_at = EXTRACT(EPOCH FROM NOW()) * 1000
WHERE session_id = $1
RETURNING *;

-- name: DeleteSessionModelConfig :exec
DELETE FROM session_model_configs
WHERE session_id = $1;
