-- name: CreateUser :one
INSERT INTO users (
    id,
    created_at,
    updated_at,
    email,
    hashed_password
)
VALUES (
    gen_random_uuid(),
    NOW(),
    NOW(),
    $1,
    $2
)
RETURNING *;

-- name: GetUserByEmail :one
SELECT *
FROM users
WHERE email = $1;


-- name: DeleteAllUsers :exec

DELETE FROM users;

-- name: CreateChirp :one

INSERT INTO chirps (id, created_at, updated_at, body, user_id)
VALUES (
    gen_random_uuid(), NOW(), NOW(), $1, $2
)
RETURNING *;

-- name: GetAllChirps :many

SELECT * 
FROM chirps
ORDER BY created_at asc;

-- name: GetChirp :one
SELECT * 
FROM chirps
WHERE id=$1;

-- name: UpdateUserCredentials :one
UPDATE users
SET
    email = $1,
    hashed_password = $2,
    updated_at = NOW()
WHERE id = $3
RETURNING *;


-- name: DeleteChirp :exec
DELETE FROM chirps
WHERE id = $1;

-- name: UpgradeUserToChirpyRed :one
UPDATE users
SET is_chirpy_red = TRUE,
    updated_at = NOW()
WHERE id = $1
RETURNING *;


-- name: GetChirpsByAuthor :many
SELECT *
FROM chirps
WHERE user_id = $1
ORDER BY created_at ASC;