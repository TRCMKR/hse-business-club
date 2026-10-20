#!/bin/sh
# Container entrypoint: applies /app/migrations with the goose CLI, then replaces
# itself with hbc (the app expects a migrated schema, ARCHITECTURE §1, rule 2).
# Re-runs are no-ops ("no migrations to run"). Arguments are passed to hbc.
set -eu

: "${POSTGRES_DSN:?POSTGRES_DSN is required}"
dir="${MIGRATIONS_DIR:-/app/migrations}"
wait_seconds="${MIGRATIONS_WAIT_SECONDS:-30}"

# sh is PID 1 until exec: without a trap SIGTERM is ignored and `docker stop` ends in SIGKILL.
trap 'exit 143' TERM INT

# Wait for the database (it may still be starting): one attempt per second. Errors
# the server reports (wrong password, unknown database or role) fail at once.
attempt=1
while ! out=$(goose -dir "$dir" postgres "$POSTGRES_DSN" status 2>&1); do
	case "$out" in
	*"starting up"* | *"shutting down"*) ;; # FATAL while Postgres starts: keep waiting
	*"password authentication failed"* | *"does not exist"* | *FATAL*)
		echo "entrypoint: database rejected the connection:" >&2
		echo "$out" >&2
		exit 1
		;;
	esac
	if [ "$attempt" -eq 1 ]; then
		echo "entrypoint: waiting for database (up to ${wait_seconds}s)" >&2
	fi
	if [ "$attempt" -ge "$wait_seconds" ]; then
		echo "entrypoint: database is not reachable after ${wait_seconds}s:" >&2
		echo "$out" >&2
		exit 1
	fi
	attempt=$((attempt + 1))
	sleep 1 &
	wait $! # interruptible, so the trap fires at once
done
trap - TERM INT

goose -dir "$dir" postgres "$POSTGRES_DSN" up
exec /usr/local/bin/hbc "$@"
