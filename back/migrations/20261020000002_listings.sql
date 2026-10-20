-- +goose Up
-- Context "listings": its own schema, fully qualified names.
-- author_id references users.users(id) logically; no cross-schema FK (ARCHITECTURE §1).
CREATE SCHEMA IF NOT EXISTS listings;

CREATE TABLE listings.listings (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    author_id    uuid NOT NULL,
    kind         text NOT NULL CHECK (kind IN ('startup', 'vacancy')),
    title        text NOT NULL,
    description  text NOT NULL,
    looking_for  text NOT NULL,
    tags         text[] NOT NULL DEFAULT '{}',
    stage        text CHECK (stage IN ('idea', 'prototype', 'mvp', 'launched')),
    compensation text NOT NULL DEFAULT '',
    status       text NOT NULL DEFAULT 'active' CHECK (status IN ('active', 'closed')),
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now(),
    closed_at    timestamptz
);
CREATE INDEX listings_feed_idx   ON listings.listings (status, created_at DESC);
CREATE INDEX listings_author_idx ON listings.listings (author_id, created_at DESC);

-- +goose Down
DROP SCHEMA IF EXISTS listings CASCADE;
