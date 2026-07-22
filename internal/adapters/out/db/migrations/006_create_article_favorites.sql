-- +goose Up
CREATE TABLE IF NOT EXISTS article_favorites (
    user_id    INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    article_id INTEGER NOT NULL REFERENCES articles(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, article_id)
);

-- +goose Down
DROP TABLE IF EXISTS article_favorites;
