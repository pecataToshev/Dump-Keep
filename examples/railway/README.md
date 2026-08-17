# Railway Deployment (Image)

Deploy dump-keep on Railway using the pre-built Docker image — no repo, no fork, no build step.

> **Note:** Dump-Keep is a one-shot tool, not a long-running service. It runs once, completes the backup, and exits. Railway's cron scheduler triggers it on schedule — you don't need to keep it running.

## Setup

1. Create a new **empty** service in your Railway project.
2. Set the service source to the Docker image: `ghcr.io/pecatatoshev/dump-keep:latest`
3. Configure as a **cron service** with schedule `17 3 * * *` (03:17 UTC nightly, or your preferred time).
4. Add environment variables (see below).
5. Deploy.

## Environment variables

### Connecting to a Railway Postgres instance

If your Postgres is also on Railway, use the reference variable:

```
POSTGRES_URL=${{Postgres.POSTGRES_URL}}
```

This connects over Railway's private network — the database is never exposed publicly.

### Full variable list

See the root [`.env.example`](../../.env.example) for all variables. The minimum for a Railway deployment:

| Variable          | Value                                        |
| ----------------- | -------------------------------------------- |
| `POSTGRES_URL`    | `${{Postgres.POSTGRES_URL}}`                 |
| `AGE_RECIPIENT`   | `age1...` public key                         |
| `STORAGE_BACKEND` | `gdrive` or `s3`                             |
| _(backend vars)_  | See `.env.example` for the backend you chose |

### Recommended optional variables

| Variable              | Why                                                                                                                         |
| --------------------- | --------------------------------------------------------------------------------------------------------------------------- |
| `HEALTHCHECK_URL`     | Railway cron failures are silent by default. Set up a free [healthchecks.io](https://healthchecks.io) check to get alerted. |
| `DISCORD_WEBHOOK_URL` | Get notified in Discord on failures and weekly/monthly success heartbeats                                                   |
| `SLACK_WEBHOOK_URL`   | Same, for Slack                                                                                                             |

## Generating an age key pair

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

- The printed **public key** (`age1...`) → `AGE_RECIPIENT` variable.
- The **private key file** → store offline (password manager, printed copy). It is never used by the service. **Without it, backups are unreadable.**

## Want a version-controlled skip file?

If you want to manage your skip list in git alongside a `railway.json`, see [examples/railway-with-config-file/](../railway-with-config-file/) — it includes a `Dockerfile` that layers your skip file on top of the published image.

## Verifying

After the first cron run (or a manual trigger), check your storage backend for the first backup folder. The service logs JSON to stdout — view logs in Railway's deploy panel.
