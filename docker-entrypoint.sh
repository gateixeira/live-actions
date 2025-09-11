#!/bin/sh
set -e

: "${WEBHOOK_SECRET:?WEBHOOK_SECRET must be set}"
: "${DATABASE_URL:?DATABASE_URL must be set}"

until psql "$DATABASE_URL" -c '\q'; do
  >&2 echo "Postgres is unavailable - sleeping"
  sleep 1
done

>&2 echo "Postgres is up - executing command"

exec "$@"