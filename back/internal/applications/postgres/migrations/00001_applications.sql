-- +goose Up
-- Runs with search_path = applications (db.Migrator). No cross-schema FKs (ARCHITECTURE §1, rule 2):
-- listing_id, listing_author_id and applicant_id are plain uuids pointing into other contexts.
CREATE TABLE applications (
    id                uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    listing_id        uuid NOT NULL,
    -- denormalized from listings at creation time (the author of a listing never changes),
    -- so access checks and "received" lists need no cross-context join
    listing_author_id uuid NOT NULL,
    applicant_id      uuid NOT NULL,
    message           text NOT NULL,
    status            text NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending', 'accepted', 'declined', 'withdrawn')),
    created_at        timestamptz NOT NULL DEFAULT now(),
    updated_at        timestamptz NOT NULL DEFAULT now(),
    decided_at        timestamptz
);
-- one "live" application per (listing, applicant); after withdraw one can apply again
CREATE UNIQUE INDEX applications_active_uniq
    ON applications (listing_id, applicant_id) WHERE status <> 'withdrawn';
CREATE INDEX applications_applicant_idx ON applications (applicant_id, created_at DESC);
CREATE INDEX applications_listing_idx   ON applications (listing_id, status, created_at DESC);
CREATE INDEX applications_listing_author_idx ON applications (listing_author_id, status, created_at DESC);

-- +goose Down
DROP TABLE applications;
