# Security Policy

## Reporting a vulnerability

If you discover a security vulnerability in Dump-Keep, please report it responsibly:

1. **Do not** open a public GitHub issue
2. Email the maintainer at: **[pecata.toshev@gmail.com]**
3. Include a description of the vulnerability and steps to reproduce if possible

You will receive a response within 48 hours. If the vulnerability is confirmed, a fix will be prioritized and a GitHub Security Advisory may be published.

## Security model

Dump-Keep is designed with defense in depth:

- **Encryption at source**: Backups are encrypted with [age](https://age-encryption.org) using a public key. The service only holds the public key (`AGE_RECIPIENT`) — the private key never touches the service. A full compromise of the service environment cannot decrypt any backup.
- **Streamed processing**: `pg_dump` output is piped through `age` encryption directly to storage. Dumps are never written to disk unencrypted.
- **Password redaction**: Notification messages have passwords in connection URLs automatically redacted before sending.
- **No inbound ports**: The service is a cron job — it makes outbound connections only (PostgreSQL, storage API, notification webhooks, healthcheck pings).

## What is NOT in scope

- **Your private key**: If your age private key is compromised, all backups encrypted to that key are compromised. Store it offline (password manager, hardware token, printed copy).
- **Your storage credentials**: If your GDrive service account JSON or S3 access keys are leaked, an attacker can delete or overwrite backups. Rotate credentials immediately if leaked.
- **Your PostgreSQL superuser password**: The service needs superuser access for `pg_dumpall --globals-only`. If this credential is leaked, an attacker can access your database directly — this is a PostgreSQL security concern, not a Dump-Keep one.

## Best practices

- Store the age private key offline, never on the backup service
- Use a dedicated PostgreSQL role for the backup service if your setup allows it
- Use separate S3 credentials scoped to a single bucket
- Enable healthchecks.io pinging to detect silent failures
- Configure notifications (Discord/Slack) to get alerted on failures
- Regularly test restoring from backups — an untested backup is not a backup
