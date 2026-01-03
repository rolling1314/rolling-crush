-- name: CreateProject :one
INSERT INTO projects (
    id,
    user_id,
    name,
    description,
    host,
    port,
    workspace_path,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7,
    EXTRACT(EPOCH FROM NOW()) * 1000,
    EXTRACT(EPOCH FROM NOW()) * 1000
)
RETURNING *;

-- name: GetProjectByID :one
SELECT *
FROM projects
WHERE id = $1 LIMIT 1;

-- name: ListProjectsByUser :many
SELECT *
FROM projects
WHERE user_id = $1
ORDER BY updated_at DESC;

-- name: UpdateProject :one
UPDATE projects
SET
    name = $2,
    description = $3,
    host = $4,
    port = $5,
    workspace_path = $6,
    updated_at = EXTRACT(EPOCH FROM NOW()) * 1000
WHERE id = $1
RETURNING *;

-- name: DeleteProject :exec
DELETE FROM projects
WHERE id = $1;

-- name: GetProjectSessions :many
SELECT s.*
FROM sessions s
WHERE s.project_id = $1
AND s.parent_session_id IS NULL
ORDER BY s.created_at DESC;

