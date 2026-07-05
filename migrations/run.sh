#!/bin/sh
set -e

MIGRATIONS_DIR="/migrations"

for script in $(ls "$MIGRATIONS_DIR"/*.sql | sort); do
    echo "Running $script..."
    psql "$DATABASE_URL" -f "$script"
    echo "Done: $script"
done

echo "All migrations completed."