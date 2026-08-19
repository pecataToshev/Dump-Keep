#!/bin/bash
set -euo pipefail

# ── Start PostgreSQL (official entrypoint handles init) ───────────
docker-entrypoint.sh postgres &
POSTGRES_PID=$!

echo "Waiting for PostgreSQL to start…"
until pg_isready -U postgres -q; do
  sleep 1
done
echo "PostgreSQL is ready."

# ── Locate the age private key ────────────────────────────────────
KEY_FILE=$(ls /data/*key*.txt 2>/dev/null | head -1 || true)
if [ -z "$KEY_FILE" ]; then
  echo "❌ No key file (*key*.txt) found in /data"
  exit 1
fi
echo "Key file: $KEY_FILE"

# ── Apply globals (roles) if present ──────────────────────────────
GLOBALS_FILE=$(ls /data/*globals.sql.age 2>/dev/null | head -1 || true)
if [ -n "$GLOBALS_FILE" ]; then
  echo "Decrypting and applying globals: $GLOBALS_FILE"
  age -d -i "$KEY_FILE" "$GLOBALS_FILE" | psql -U postgres 2>&1 || true
  echo "Globals applied (role-already-exists errors are expected)."
fi

# ── Process every encrypted dump in /data ─────────────────────────
DUMP_FILES=$(ls /data/*.dump.age 2>/dev/null || true)
if [ -z "$DUMP_FILES" ]; then
  echo "❌ No encrypted dump file (*.dump.age) found in /data"
  exit 1
fi

for DUMP_FILE in $DUMP_FILES; do
  echo ""
  echo "━━━ Processing: $(basename "$DUMP_FILE") ━━━"

  # Extract DB name from filename: <yyyy-MM-dd>-<dbname>.dump.age
  BASENAME=$(basename "$DUMP_FILE")
  DB_NAME=$(echo "$BASENAME" | sed -E 's/^[0-9]{4}-[0-9]{2}-[0-9]{2}-(.+)\.dump\.age$/\1/')
  if [ -z "$DB_NAME" ] || [ "$DB_NAME" = "$BASENAME" ]; then
    DB_NAME="restored"
  fi
  echo "Target database: $DB_NAME"

  # Decrypt
  echo "Decrypting…"
  age -d -i "$KEY_FILE" "$DUMP_FILE" > /tmp/restore.dump

  # Drop & recreate the database for a clean restore
  psql -U postgres -c "DROP DATABASE IF EXISTS \"$DB_NAME\";"
  psql -U postgres -c "CREATE DATABASE \"$DB_NAME\";"

  # Restore (custom-format dump)
  echo "Restoring…"
  pg_restore -U postgres -d "$DB_NAME" --no-owner --no-privileges /tmp/restore.dump 2>&1 || true
  rm /tmp/restore.dump

  # ── Validate: tables exist and at least one has rows ────────────
  echo "Validating…"
  TABLE_COUNT=$(psql -U postgres -d "$DB_NAME" -t -A -c \
    "SELECT count(*) FROM information_schema.tables
     WHERE table_schema NOT IN ('pg_catalog','information_schema')
       AND table_type = 'BASE TABLE';")

  if [ "${TABLE_COUNT:-0}" -eq 0 ] 2>/dev/null; then
    echo "❌ Validation failed: no user tables in $DB_NAME"
    exit 1
  fi
  echo "  $TABLE_COUNT user table(s) found."

  HAS_DATA=false
  for table in $(psql -U postgres -d "$DB_NAME" -t -A -c \
    "SELECT quote_ident(table_schema)||'.'||quote_ident(table_name)
     FROM information_schema.tables
     WHERE table_schema NOT IN ('pg_catalog','information_schema')
       AND table_type = 'BASE TABLE';"); do
    has_rows=$(psql -U postgres -d "$DB_NAME" -t -A -c \
      "SELECT EXISTS (SELECT 1 FROM $table);" 2>/dev/null || echo "f")
    if [ "$has_rows" = "t" ]; then
      row_count=$(psql -U postgres -d "$DB_NAME" -t -A -c \
        "SELECT count(*) FROM $table;" 2>/dev/null || echo "?")
      echo "  ✓ $table — $row_count rows"
      HAS_DATA=true
    else
      echo "  · $table — empty"
    fi
  done

  if [ "$HAS_DATA" = false ]; then
    echo "❌ Validation failed: no tables with data in $DB_NAME"
    exit 1
  fi
  echo "✅ $DB_NAME restored and validated."
done

# ── Shut down PostgreSQL cleanly ──────────────────────────────────
echo ""
echo "Stopping PostgreSQL…"
su postgres -c "pg_ctl -D '$PGDATA' stop -m fast"
wait "$POSTGRES_PID" 2>/dev/null || true

echo ""
echo "✅ All dumps restored and validated. Container will be removed."
