-- name: GetFile :one
SELECT *
FROM files
WHERE id = $1 LIMIT 1;

-- name: GetFileByPathAndSession :one
SELECT *
FROM files
WHERE path = $1 AND session_id = $2
ORDER BY version DESC, created_at DESC
LIMIT 1;

-- name: ListFilesBySession :many
SELECT *
FROM files
WHERE session_id = $1
ORDER BY version ASC, created_at ASC;

-- name: ListFilesByPath :many
SELECT *
FROM files
WHERE path = $1
ORDER BY version DESC, created_at DESC;

-- name: CreateFile :one
INSERT INTO files (
    id,
    session_id,
    path,
    content,
    version,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, EXTRACT(EPOCH FROM NOW()) * 1000, EXTRACT(EPOCH FROM NOW()) * 1000
)
RETURNING *;

-- name: DeleteFile :exec
DELETE FROM files
WHERE id = $1;

-- name: DeleteSessionFiles :exec
DELETE FROM files
WHERE session_id = $1;

-- name: ListLatestSessionFiles :many
SELECT f.*
FROM files f
INNER JOIN (
    SELECT path, MAX(version) as max_version, MAX(created_at) as max_created_at
    FROM files
    GROUP BY path
) latest ON f.path = latest.path AND f.version = latest.max_version AND f.created_at = latest.max_created_at
WHERE f.session_id = $1
ORDER BY f.path;

-- name: ListNewFiles :many
SELECT *
FROM files
WHERE is_new = 1
ORDER BY version DESC, created_at DESC;
