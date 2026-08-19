# Restore Test

Validates an encrypted [dump-keep](../README.md) backup end-to-end: spin
up a throwaway PostgreSQL container, decrypt the dump with the offline
age key, restore it, check that data is present, then remove the
container.

This automates the "test a restore quarterly" step every backup strategy
should include — without touching your production database.

## Prerequisites

- Docker (with the Compose plugin)
- The offline age private key (`dump-keep-key.txt` — see
  [Generate an encryption key pair](../README.md#1-generate-an-encryption-key-pair)
  in the main README)
- One or more encrypted dump files produced by dump-keep and downloaded
  from your storage backend (`<yyyy-MM-dd>-<dbname>.dump.age`)
- Optionally the globals file (`<yyyy-MM-dd>-globals.sql.age`) to
  restore roles first

## Usage

1. Place the key and dump file(s) in `.data/`:

```
restore-test/.data/
├── dump-keep-key.txt
├── 2026-07-13-globals.sql.age   ← optional
└── 2026-07-13-log_to_db.dump.age
```

2. Run:

```bash
docker compose run --rm --build restore-test
```

Compose builds the image (`postgres:18` + age), starts a container with
`.data/` mounted read-only at `/data`, and the entrypoint:

1. Starts PostgreSQL inside the container.
2. Finds `*key*.txt` and `*.dump.age` in `/data`.
3. If `*globals.sql.age` is present, decrypts and applies it first
   (roles — "role already exists" errors are expected).
4. For each `*.dump.age`: decrypts with age, drops & recreates the
   target database (name extracted from the filename), restores with
   `pg_restore --no-owner --no-privileges`.
5. Validates: counts user tables and checks that at least one table
   has rows (`SELECT EXISTS`).
6. Stops PostgreSQL and exits.

The `--rm` flag removes the container automatically when it exits —
no state left behind. `docker compose up` also works but leaves the
container around after exit; `run --rm` is cleaner for one-shot jobs.

## Database name extraction

The dump filename pattern is `<yyyy-MM-dd>-<dbname>.dump.age` (e.g.
`2026-07-13-log_to_db.dump.age`). The database name `log_to_db` is
extracted and used as the restore target. If the pattern doesn't
match, the database is named `restored`.

## Files

| File            | Purpose                                                               |
| --------------- | --------------------------------------------------------------------- |
| `Dockerfile`    | `postgres:18` + age                                                   |
| `entrypoint.sh` | Runs inside the container: start PG, decrypt, restore, validate, stop |
| `compose.yml`   | Compose service definition: build, env, `./.data:/data` volume mount  |
| `.data/`        | Drop zone for the key and encrypted dumps (gitignored)                |
