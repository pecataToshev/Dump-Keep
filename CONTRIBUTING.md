# Contributing to Dump-Keep

Thanks for your interest in contributing! This document covers the basics.

## Development setup

```bash
git clone https://github.com/pecataToshev/Dump-Keep.git
cd Dump-Keep
go test ./...
```

You need Go 1.26+ and optionally `age` installed for local testing:

```bash
# Generate a test key pair
age-keygen -o test-key.txt
# Use the printed public key (age1...) as AGE_RECIPIENT
```

## Architecture

```
main.go                      # Entry point — wires config, storage, notifications
internal/
  config/                    # Env var loading + validation
  storage/                   # Provider interface + factory
    gdrive/                  # Google Drive backend
    s3/                      # S3-compatible backend (MinIO client)
  backup/                    # Backup orchestration — dump, encrypt, upload, prune
  notify/                    # Notification channels (Discord, Slack, Multi)
  healthcheck/               # healthchecks.io pinging
  buildinfo/                 # Build metadata (injected by CI)
```

### Adding a new storage backend

1. Create `internal/storage/<name>/<name>.go`
2. Implement the `storage.Provider` interface (`EnsureFolder`, `Put`, `Delete`, `ListFolders`, `DeleteFolder`)
3. Validate backend-specific config in your `New(cfg)` constructor
4. Add a `case` in `internal/storage/storage.go` → `New()`
5. Add config fields to `internal/config/config.go` if needed
6. Document env vars in `.env.example` and `README.md`

### Adding a notification channel

1. Create `internal/notify/<name>.go`
2. Implement the `Notifier` interface (`Notify(message string) error`)
3. Wire it in `buildNotifier()` in `main.go`
4. Document the env var in `.env.example` and `README.md`

## Code style

- Follow standard Go conventions (`gofmt`, `go vet`)
- Keep functions focused — the codebase values simplicity
- Don't add comments unless the code is non-obvious
- Use `slog` for logging (JSON handler, no timestamps — Railway injects them)
- Stream large data — never buffer entire dumps in memory

## Testing

```bash
go test ./...
go vet ./...
```

When adding a feature, add tests for the new behavior. Mock the `storage.Provider` interface in backup tests — see `backup_test.go` for the pattern.

## Pull requests

1. Fork the repo and create a branch from `main`
2. Run `go test ./...` and `go vet ./...` — both must pass
3. Keep PRs focused — one feature or fix per PR
4. Write a clear PR description explaining what and why

## Reporting bugs

Open a [GitHub issue](https://github.com/pecataToshev/Dump-Keep/issues) with:
- What you expected
- What happened
- Your configuration (redact secrets!)
- Logs (redact secrets — the service redacts passwords in notifications, but raw logs may contain connection strings)
