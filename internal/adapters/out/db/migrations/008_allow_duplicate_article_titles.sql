-- +goose Up
ALTER TABLE articles DROP CONSTRAINT IF EXISTS articles_title_unique;

-- +goose Down
ALTER TABLE articles ADD CONSTRAINT articles_title_unique UNIQUE (title);
