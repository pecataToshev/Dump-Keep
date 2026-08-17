# Dump-Keep

Encrypted PostgreSQL backups shipped to Google Drive or any S3-compatible storage. Runs on a cron, encrypts with [age](https://age-encryption.org), retains by tier.

## How it works

Dump-Keep is a **one-shot CLI tool** — it starts, backs up all databases, uploads them, prunes old backups, and exits. It is **not a long-running daemon**. Schedule it with a cron runner (Railway cron, Kubernetes CronJob, systemd timer, GitHub Actions schedule, etc.) to trigger periodic runs.

Each invocation:

1. Connects to PostgreSQL, enumerates all non-template databases
2. Dumps globals (`pg_dumpall --globals-only`) and each database (`pg_dump -Fc`)
3. Streams each dump through `age` encryption directly to storage — no unencrypted data touches disk
4. Prunes old backup folders based on tier retention
5. Sends notifications on failure (and weekly/monthly success heartbeats)
6. Exits

## Features

- **PostgreSQL 15–18** — dumps every non-template database on the instance, automatically picks up new ones
- **Encrypted at source** — uses `age` in recipient mode; the service only holds the public key, so a leak of its environment cannot decrypt any backup
- **Two storage backends** — Google Shared Drive or any S3-compatible storage (AWS S3, MinIO, Backblaze B2, Cloudflare R2, etc.)
- **Tiered retention** — daily / weekly / monthly folders with configurable retention, no duplicate copies
- **Streamed** — `pg_dump` → `age` encrypt → upload, without touching disk
- **Notifications** — Discord and/or Slack webhooks for failures and weekly/monthly heartbeats
- **Health checks** — optional [healthchecks.io](https://healthchecks.io) pinging for silent-failure detection
- **Configurable skip list** — skip specific databases via env var or file

## Quick start

### 1. Generate an encryption key pair

Backups are encrypted with [age](https://age-encryption.org). You need a key pair — the public key goes to the service, the private key stays offline.

**Using Docker** (no need to install age locally):

```bash
docker run --rm -v "$PWD:/out" --entrypoint age-keygen \
  jauderho/age:latest -o /out/dump-keep-key.txt
```

**Or install age directly:**

```bash
# macOS:  brew install age
# Linux:  apt install age  (or download from https://age-encryption.org)
age-keygen -o dump-keep-key.txt
```

The command prints the public key (`age1...`) and writes the private key to `dump-keep-key.txt`. Store the private key offline — **without it, backups are unreadable**.

### 2. Run a backup

Pull the pre-built image from GHCR and run with env vars — no fork or clone needed:

```bash
docker run --rm \
  -e POSTGRES_URL=postgresql://user:pass@host:5432/postgres \
  -e AGE_RECIPIENT=age1xxxxxxxxxxxxxxxxxxxxxxxxxxxxx \
  -e STORAGE_BACKEND=s3 \
  -e S3_ENDPOINT=https://s3.amazonaws.com \
  -e S3_BUCKET=my-backups \
  -e S3_REGION=us-east-1 \
  -e S3_ACCESS_KEY=AKIAIOSFODNN7EXAMPLE \
  -e S3_SECRET_KEY=wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY \
  ghcr.io/pecatatoshev/dump-keep:latest
```

For scheduled runs, deploy as a cron job — see [examples/](./examples) for Railway and other platforms.

## Backup layout

Each run creates **one folder** named `<yyyy-MM-dd>_<tier>_<HHMMSS>` inside the configured storage root, containing:

| Artifact                       | Source                      | Purpose                               |
| ------------------------------ | --------------------------- | ------------------------------------- |
| `<yyyy-MM-dd>-globals.sql.age` | `pg_dumpall --globals-only` | All roles + passwords                 |
| `<yyyy-MM-dd>-<db>.dump.age`   | `pg_dump -Fc` per database  | Each database individually restorable |

Databases are enumerated from `pg_database` at runtime, so newly created databases are picked up automatically.

### Tier logic

| Tier    | When         | Default retention |
| ------- | ------------ | ----------------- |
| Daily   | Most days    | 7 days            |
| Weekly  | Sundays      | 4 weeks           |
| Monthly | 1st of month | 24 months         |

Retention is encoded in the folder name — a weekly folder simply lives longer than a daily one. No copies are made. Configure via `RETENTION` (e.g. `RETENTION=14d,8w,12m`) or disable with `RETENTION=none`.

## Configuration

All configuration is via environment variables. See [`.env.example`](./.env.example) for the full reference.

### Required

| Variable          | Description                 |
| ----------------- | --------------------------- |
| `POSTGRES_URL`    | Superuser connection string |
| `AGE_RECIPIENT`   | age public key (`age1...`)  |
| `STORAGE_BACKEND` | `gdrive` or `s3`            |

### Google Drive (`STORAGE_BACKEND=gdrive`)

| Variable                 | Description                                        |
| ------------------------ | -------------------------------------------------- |
| `GDRIVE_SA_JSON`         | Service account JSON (full content)                |
| `GDRIVE_SHARED_DRIVE_ID` | Shared Drive ID from the folder URL                |
| `GDRIVE_FOLDER_ID`       | _(optional)_ Parent folder ID, default: drive root |

See [docs/gdrive-setup.md](./docs/gdrive-setup.md) for setup instructions.

### S3 (`STORAGE_BACKEND=s3`)

| Variable        | Description                                                   |
| --------------- | ------------------------------------------------------------- |
| `S3_ENDPOINT`   | Endpoint URL (e.g. `https://s3.amazonaws.com`)                |
| `S3_BUCKET`     | Bucket name (must already exist)                              |
| `S3_REGION`     | Region (e.g. `us-east-1`)                                     |
| `S3_ACCESS_KEY` | Access key                                                    |
| `S3_SECRET_KEY` | Secret key                                                    |
| `S3_PATH_STYLE` | _(optional)_ `true` for MinIO and some S3-compatible storages |
| `S3_PREFIX`     | _(optional)_ Key prefix (e.g. `backups/postgres/`)            |

### Optional

| Variable                   | Description                                                                                                 |
| -------------------------- | ----------------------------------------------------------------------------------------------------------- |
| `SKIP_DATABASES`           | Comma-separated database names to skip                                                                      |
| `SKIP_DATABASES_FILE_PATH` | Path to a skip list file (one per line, `#` = comment). Merged with `SKIP_DATABASES`                        |
| `RETENTION`                | Retention durations as `daily,weekly,monthly` (e.g. `7d,4w,24m`). `none` disables pruning. Unset = defaults |
| `HEALTHCHECK_URL`          | healthchecks.io ping URL                                                                                    |
| `DISCORD_WEBHOOK_URL`      | Discord webhook for failure/heartbeat notifications                                                         |
| `SLACK_WEBHOOK_URL`        | Slack webhook for failure/heartbeat notifications                                                           |

## Restoring

Backups are encrypted with `age` and compressed in PostgreSQL custom format. To restore:

```bash
# Decrypt and restore globals
age -d -i private-key.txt 2026-08-17-globals.sql.age | psql -h host -U postgres

# Decrypt and restore a single database
age -d -i private-key.txt 2026-08-17-mydb.dump.age | pg_restore -h host -U postgres -d mydb --clean --if-exists
```

## Examples

- [examples/railway/](./examples/railway) — Deploy on Railway using the pre-built image (simplest)
- [examples/railway-with-config-file/](./examples/railway-with-config-file) — Railway deployment with a version-controlled skip file

## Development

```bash
# Run tests
go test ./...

# Build
go build -o dump-keep .

# Run locally
set -a; . ./.env; set +a; go run .
```

See [CONTRIBUTING.md](./CONTRIBUTING.md) for contribution guidelines.

## License

[MIT](./LICENSE)
