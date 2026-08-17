# Railway Deployment with Config File

This example shows how to deploy dump-keep on Railway with a version-controlled skip file, using the pre-built Docker image — no need to fork the main repo.

> **Note:** Dump-Keep is a one-shot tool, not a long-running service. It runs once, completes the backup, and exits. Railway's cron scheduler triggers it on schedule — you don't need to keep it running.

## What's in this folder

| File           | Purpose                                             |
| -------------- | --------------------------------------------------- |
| `Dockerfile`   | Layers your skip file on top of the published image |
| `skip.txt`     | Your list of databases to skip (edit this)          |
| `railway.json` | Railway cron schedule and deploy config             |

## Setup

### 1. Create your own repo

Copy the contents of this folder into a new GitHub repo:

```bash
mkdir my-dump-keep && cd my-dump-keep
cp -r /path/to/Dump-Keep/examples/railway-with-config-file/* .
git init && git add . && git commit -m "initial dump-keep config"
git remote add origin git@github.com:youruser/my-dump-keep.git
git push -u origin main
```

### 2. Edit the skip file

Open `skip.txt` and add any databases you want to exclude from backups. One per line, `#` for comments. See the file for examples.

### 3. Generate an age key pair

**Using Docker:**

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

Store the private key file offline — **without it, backups are unreadable**.

### 4. Deploy on Railway

1. Create a new Railway service from your GitHub repo (`youruser/my-dump-keep`).
2. Railway will detect the `Dockerfile` and `railway.json` automatically.
3. The cron schedule is set in `railway.json` (`17 3 * * *` — 03:17 UTC nightly).
4. Add your environment variables (see below).
5. Deploy.

### 5. Environment variables

Set these in the Railway service variables panel:

| Variable          | Value                                                                              |
| ----------------- | ---------------------------------------------------------------------------------- |
| `POSTGRES_URL`    | `${{Postgres.POSTGRES_URL}}` (if Postgres is on Railway) or your connection string |
| `AGE_RECIPIENT`   | `age1...` public key from step 3                                                   |
| `STORAGE_BACKEND` | `gdrive` or `s3`                                                                   |

Plus the backend-specific variables — see the root [`.env.example`](../../.env.example) for the full list.

### Recommended optional variables

| Variable              | Why                                                                        |
| --------------------- | -------------------------------------------------------------------------- |
| `RETENTION`           | e.g. `7d,4w,24m` (default) or `14d,8w,12m` or `none` to disable pruning    |
| `HEALTHCHECK_URL`     | Detect silent cron failures via [healthchecks.io](https://healthchecks.io) |
| `DISCORD_WEBHOOK_URL` | Get notified in Discord on failures and weekly/monthly heartbeats          |
| `SLACK_WEBHOOK_URL`   | Same, for Slack                                                            |

## Updating the skip list

Just edit `skip.txt`, commit, and push. Railway will rebuild the image with the new skip list on the next deploy.

## How it works

The `Dockerfile` is one line of real work:

```dockerfile
FROM ghcr.io/pecatatoshev/dump-keep:latest
COPY skip.txt /skip.txt
ENV SKIP_DATABASES_FILE_PATH=/skip.txt
```

It pulls the latest published dump-keep image, copies your skip file into it, and sets the env var to point at it. You get the latest dump-keep features without maintaining any Go code.
