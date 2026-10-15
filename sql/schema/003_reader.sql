-- +goose Up
CREATE TABLE feeds (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    url             TEXT NOT NULL UNIQUE,
    site_url        TEXT,
    title           TEXT NOT NULL DEFAULT '',
    description     TEXT NOT NULL DEFAULT '',
    favicon         TEXT,
    etag            TEXT,
    last_modified   TEXT,
    last_fetched_at TIMESTAMPTZ,
    next_fetch_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    error_count     INT NOT NULL DEFAULT 0,
    last_error      TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX feeds_next_fetch_idx ON feeds (next_fetch_at);

CREATE TABLE subscriptions (
    user_id      TEXT NOT NULL REFERENCES "user" ("id") ON DELETE CASCADE,
    feed_id      BIGINT NOT NULL REFERENCES feeds (id) ON DELETE CASCADE,
    folder       TEXT,
    custom_title TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, feed_id)
);
CREATE INDEX subscriptions_feed_idx ON subscriptions (feed_id);

CREATE TABLE articles (
    id           BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    feed_id      BIGINT NOT NULL REFERENCES feeds (id) ON DELETE CASCADE,
    guid         TEXT NOT NULL,
    url          TEXT NOT NULL,
    title        TEXT NOT NULL,
    author       TEXT,
    published_at TIMESTAMPTZ NOT NULL,
    summary      TEXT NOT NULL DEFAULT '',
    content_html TEXT NOT NULL DEFAULT '',
    content_text TEXT NOT NULL DEFAULT '',
    image_url    TEXT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    search_tsv   TSVECTOR GENERATED ALWAYS AS (
        setweight(to_tsvector('english', coalesce(title, '')), 'A') ||
        setweight(to_tsvector('english', left(coalesce(content_text, ''), 20000)), 'B')
    ) STORED,
    UNIQUE (feed_id, guid)
);
CREATE INDEX articles_feed_published_idx ON articles (feed_id, published_at DESC, id DESC);
CREATE INDEX articles_search_idx ON articles USING GIN (search_tsv);

CREATE TABLE user_article (
    user_id    TEXT NOT NULL REFERENCES "user" ("id") ON DELETE CASCADE,
    article_id BIGINT NOT NULL REFERENCES articles (id) ON DELETE CASCADE,
    read_at    TIMESTAMPTZ,
    saved      BOOLEAN NOT NULL DEFAULT FALSE,
    rating     SMALLINT NOT NULL DEFAULT 0 CHECK (rating BETWEEN -1 AND 1),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, article_id)
);
CREATE INDEX user_article_saved_idx ON user_article (user_id) WHERE saved;

-- +goose Down
DROP TABLE user_article;
DROP TABLE articles;
DROP TABLE subscriptions;
DROP TABLE feeds;
