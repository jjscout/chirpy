-- name: AddRefreshToken :one
INSERT INTO refresh_tokens (
    token,
    created_at,
    updated_at,
    user_id,
    expires_at,
    revoked_at
) VALUES (
    $1,
    NOW(),
    NOW(),
    $2,
    NOW() + make_interval(hours => @expires_in),
    NULL
)
RETURNING *;
-- name: RevokeRefreshToken :one
UPDATE
    refresh_tokens
SET
    revoked_at = NOW()
WHERE
    token = $1 RETURNING *;