-- name: CreateUser :one
INSERT INTO users (
    id,
    username,
    email,
    password_hash,
    avatar_url,
    created_at,
    updated_at
) VALUES (
    $1, $2, $3, $4, $5, 
    EXTRACT(EPOCH FROM NOW()) * 1000, 
    EXTRACT(EPOCH FROM NOW()) * 1000
)
RETURNING *;

-- name: GetUserByID :one
SELECT *
FROM users
WHERE id = $1 LIMIT 1;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1 LIMIT 1;

-- name: GetUserByUsername :one
SELECT *
FROM users
WHERE username = $1 LIMIT 1;

-- name: UpdateUser :one
UPDATE users
SET
    username = $2,
    email = $3,
    avatar_url = $4,
    updated_at = EXTRACT(EPOCH FROM NOW()) * 1000
WHERE id = $1
RETURNING *;

-- name: UpdateUserPassword :exec
UPDATE users
SET
    password_hash = $2,
    updated_at = EXTRACT(EPOCH FROM NOW()) * 1000
WHERE id = $1;

-- name: DeleteUser :exec
DELETE FROM users
WHERE id = $1;

