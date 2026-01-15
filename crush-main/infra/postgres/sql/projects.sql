-- name: CreateProject :one
INSERT INTO projects (
    id,
    user_id,
    name,
    description,
    external_ip,
    frontend_port,
    workspace_path,
    container_name,
    workdir_path,
    db_host,
    db_port,
    db_user,
    db_password,
    db_name,
    backend_port,
    frontend_command,
    frontend_language,
    backend_command,
    backend_language,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19,
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
    external_ip = $4,
    frontend_port = $5,
    workspace_path = $6,
    container_name = $7,
    workdir_path = $8,
    db_host = $9,
    db_port = $10,
    db_user = $11,
    db_password = $12,
    db_name = $13,
    backend_port = $14,
    frontend_command = $15,
    frontend_language = $16,
    backend_command = $17,
    backend_language = $18,
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

