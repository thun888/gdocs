-- +goose Up
CREATE TABLE users (
    id            bigserial PRIMARY KEY,
    subject       varchar(32)  NOT NULL UNIQUE,
    username      varchar(64)  NOT NULL,
    display_name  varchar(128) NOT NULL DEFAULT '',
    email         varchar(255) NOT NULL DEFAULT '',
    class         varchar(128) NOT NULL DEFAULT '',
    role          varchar(16)  NOT NULL DEFAULT 'viewer',
    created_at    timestamptz  NOT NULL DEFAULT now(),
    last_login_at timestamptz
);

CREATE TABLE "groups" (
    id           bigserial PRIMARY KEY,
    code         varchar(128) NOT NULL UNIQUE,
    name         varchar(64)  NOT NULL,
    type         varchar(16)  NOT NULL DEFAULT '',
    role         varchar(16)  NOT NULL DEFAULT 'viewer',
    ignored      boolean      NOT NULL DEFAULT false,
    last_seen_at timestamptz
);

CREATE TABLE user_groups (
    user_id  bigint NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    group_id bigint NOT NULL REFERENCES "groups"(id) ON DELETE CASCADE,
    PRIMARY KEY (user_id, group_id)
);
CREATE INDEX user_groups_group_id_idx ON user_groups (group_id);

CREATE TABLE sessions (
    id                varchar(64) PRIMARY KEY,
    uid               bigint      NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    access_token_enc  bytea,
    refresh_token_enc bytea,
    token_expires_at  timestamptz NOT NULL DEFAULT now(),
    expires_at        timestamptz NOT NULL,
    last_seen_at      timestamptz NOT NULL DEFAULT now(),
    ip                varchar(64) NOT NULL DEFAULT '',
    ua                text        NOT NULL DEFAULT ''
);
CREATE INDEX sessions_uid_idx ON sessions (uid);
CREATE INDEX sessions_expires_at_idx ON sessions (expires_at);

CREATE TABLE sources (
    id         bigserial PRIMARY KEY,
    slug       varchar(64)  NOT NULL UNIQUE,
    name       varchar(128) NOT NULL,
    base_url   varchar(255) NOT NULL,
    token_enc  bytea,
    owner      varchar(128) NOT NULL,
    repo       varchar(128) NOT NULL,
    branch     varchar(128) NOT NULL DEFAULT 'main',
    root       varchar(255) NOT NULL DEFAULT '',
    sort       int          NOT NULL DEFAULT 0,
    enabled    boolean      NOT NULL DEFAULT true,
    created_at timestamptz  NOT NULL DEFAULT now()
);

CREATE TABLE doc_meta (
    source_id  bigint        NOT NULL REFERENCES sources(id) ON DELETE CASCADE,
    path       varchar(1024) NOT NULL,
    type       varchar(8)    NOT NULL DEFAULT 'doc',
    blob_sha   varchar(64)   NOT NULL DEFAULT '',
    title      varchar(255)  NOT NULL DEFAULT '',
    sort       int           NOT NULL DEFAULT 0,
    is_public  boolean,
    draft      boolean       NOT NULL DEFAULT false,
    updated_at timestamptz   NOT NULL DEFAULT now(),
    PRIMARY KEY (source_id, path)
);

CREATE TABLE settings (
    "key" varchar(64) PRIMARY KEY,
    value text        NOT NULL DEFAULT ''
);

CREATE TABLE audit_logs (
    id         bigserial PRIMARY KEY,
    uid        bigint REFERENCES users(id) ON DELETE SET NULL,
    action     varchar(64)  NOT NULL,
    target     varchar(255) NOT NULL DEFAULT '',
    detail     jsonb,
    ip         varchar(64)  NOT NULL DEFAULT '',
    created_at timestamptz  NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE IF EXISTS audit_logs;
DROP TABLE IF EXISTS settings;
DROP TABLE IF EXISTS doc_meta;
DROP TABLE IF EXISTS sources;
DROP TABLE IF EXISTS sessions;
DROP TABLE IF EXISTS user_groups;
DROP TABLE IF EXISTS "groups";
DROP TABLE IF EXISTS users;
