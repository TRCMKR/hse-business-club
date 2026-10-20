#!/bin/sh
# Container entrypoint: applies /app/migrations with the goose CLI, then replaces
# itself with hbc (the app expects a migrated schema, ARCHITECTURE §1, rule 2).
# Re-runs are no-ops ("no migrations to run"). Arguments are passed to hbc.
set -eu

: "${POSTGRES_DSN:?POSTGRES_DSN is required}"
dir="${MIGRATIONS_DIR:-/app/migrations}"
wait_seconds="${MIGRATIONS_WAIT_SECONDS:-30}"

# Wait for the database (it may still be starting): one attempt per second.
attempt=1
until goose -dir "$dir" postgres "$POSTGRES_DSN" status >/dev/null 2>&1; do
	if [ "$attempt" -ge "$wait_seconds" ]; then
		echo "entrypoint: database is not reachable after ${wait_seconds}s" >&2
		goose -dir "$dir" postgres "$POSTGRES_DSN" status >/dev/null # prints the error
		exit 1
	fi
	attempt=$((attempt + 1))
	sleep 1
done

goose -dir "$dir" postgres "$POSTGRES_DSN" up
exec /usr/local/bin/hbc "$@"
