-- name: CreateFeed :one
INSERT INTO feeds (id, created_at, updated_at, name, url, user_id)
VALUES (
    $1, 
    $2, 
    $3, 
    $4, 
    $5, 
    $6
)
RETURNING *;

-- name: GetFeed :one
SELECT * FROM feeds
WHERE id = $1 LIMIT 1;

-- name: GetFeedByName :one
SELECT * FROM feeds
WHERE name = $1 LIMIT 1;

-- name: GetFeedByUrl :one
SELECT * FROM feeds
WHERE url = $1 LIMIT 1;

-- name: DeleteAllFeeds :exec
DELETE FROM feeds;

-- name: GetFeeds :many
SELECT * FROM feeds; 

-- name: MarkFeedFetched :exec
UPDATE feeds
SET updated_at = $2, last_fetched_at = $2
WHERE id = $1;

-- name: GetNextFeedToFetch :one
SELECT * FROM feeds
ORDER BY last_fetched_at ASC NULLS FIRST
LIMIT 1;

