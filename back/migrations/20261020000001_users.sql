-- +goose Up
-- Context "users" (ARCHITECTURE §1, rule 2): its own schema, fully qualified names.
CREATE SCHEMA IF NOT EXISTS users;

CREATE TABLE users.users (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    username      text NOT NULL UNIQUE,
    password_hash text NOT NULL,
    display_name  text NOT NULL,
    headline      text NOT NULL DEFAULT '',
    about         text NOT NULL DEFAULT '',
    skills        text[] NOT NULL DEFAULT '{}',
    telegram      text NOT NULL DEFAULT '',
    avatar_key    text NOT NULL DEFAULT '', -- '<uuid>.jpg' in the bucket, '' = no photo
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

-- +goose Down
DROP SCHEMA IF EXISTS users CASCADE;
