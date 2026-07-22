-- +goose Up
CREATE TABLE IF NOT EXISTS tags (
    id   SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL
);
ALTER TABLE tags ADD CONSTRAINT tags_name_unique UNIQUE (name);

CREATE TABLE IF NOT EXISTS article_tags (
    article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    tag_id     INTEGER NOT NULL REFERENCES tags(id) ON DELETE CASCADE,
    PRIMARY KEY (article_id, tag_id)
);

-- +goose Down
DROP TABLE IF EXISTS article_tags;
DROP TABLE IF EXISTS tags;
