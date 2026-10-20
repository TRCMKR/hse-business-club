-- +goose Up
-- Refresh tokens of the users context (SPEC §3, §5): only the SHA-256 of a token is
-- stored. A family is the chain of rotations of one sign-in.
CREATE TABLE users.refresh_tokens (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id     uuid NOT NULL REFERENCES users.users (id) ON DELETE CASCADE,
    family_id   uuid NOT NULL,
    token_hash  bytea NOT NULL UNIQUE, -- sha256(refresh_token)
    expires_at  timestamptz NOT NULL,
    created_at  timestamptz NOT NULL DEFAULT now(),
    revoked_at  timestamptz            -- NULL = active
);
CREATE INDEX refresh_tokens_user_idx   ON users.refresh_tokens (user_id);
CREATE INDEX refresh_tokens_family_idx ON users.refresh_tokens (family_id);

-- +goose Down
DROP TABLE IF EXISTS users.refresh_tokens;
