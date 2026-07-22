-- +goose Up
CREATE TABLE IF NOT EXISTS articles (
    id          SERIAL PRIMARY KEY,
    slug        VARCHAR(255) NOT NULL,
    title       VARCHAR(255) NOT NULL,
    description TEXT NOT NULL,
    body        TEXT NOT NULL,
    author_id   INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
ALTER TABLE articles ADD CONSTRAINT articles_slug_unique UNIQUE (slug);
ALTER TABLE articles ADD CONSTRAINT articles_title_unique UNIQUE (title);

-- +goose Down
DROP TABLE IF EXISTS articles;
