-- name: GetLink :one
SELECT * FROM links
WHERE id = $1 LIMIT 1;

-- name: ListLinks :many
SELECT * FROM links
ORDER BY id
LIMIT $1 OFFSET $2;

-- name: CountLinks :one
SELECT count(*) FROM links;

-- name: GetLinkByShortName :one
SELECT * FROM links
WHERE short_name = $1 LIMIT 1;

-- name: CreateLink :one
INSERT INTO links (
    original_url,
    short_name
) VALUES (
    $1, $2
) RETURNING *;

-- name: UpdateLink :one
UPDATE links
SET 
    original_url = COALESCE(sqlc.narg('original_url'), original_url),
    short_name = COALESCE(sqlc.narg('short_name'), short_name),
    updated_at = NOW()
WHERE id = sqlc.arg('id')
RETURNING *;

-- name: DeleteLink :exec
DELETE FROM links
WHERE id = $1;
