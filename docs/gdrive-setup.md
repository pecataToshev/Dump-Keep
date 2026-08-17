# Google Drive Setup

Dump-Keep can upload backups to a Google Shared Drive using a service account. A Shared Drive is required — files uploaded by a service account into a regular shared folder would count against the account's own 15 GB quota.

## 1. Create a service account

1. Go to [console.cloud.google.com](https://console.cloud.google.com) and create a project (e.g. `dump-keep-backups`).
2. **APIs & Services → Enable APIs** → enable **Google Drive API**.
3. **IAM & Admin → Service Accounts** → create a service account (e.g. `dump-keep`). No project roles needed.
4. Open the service account → **Keys → Add key → JSON** — download the JSON file.
5. Note the service account email (e.g. `dump-keep@my-project.iam.gserviceaccount.com`).

## 2. Create a Shared Drive

1. Google Drive → **Shared drives → New** (e.g. `DB Backups`).
2. Add the service account email as a member with **Content manager** role.
   - Contributor cannot delete, so pruning would fail.
3. Copy the Shared Drive ID from its URL (`https://drive.google.com/drive/folders/<ID>`).

## 3. Configure Dump-Keep

Set these environment variables:

| Variable | Value |
|----------|-------|
| `STORAGE_BACKEND` | `gdrive` |
| `GDRIVE_SA_JSON` | Full content of the downloaded JSON file |
| `GDRIVE_SHARED_DRIVE_ID` | Shared Drive ID from the URL |
| `GDRIVE_FOLDER_ID` | *(optional)* Parent folder ID for backups. Unset = drive root |

To use a specific subfolder instead of the drive root, create a folder inside the Shared Drive, copy its ID from the URL, and set `GDRIVE_FOLDER_ID`.

## Notes

- The service account JSON contains a private key — store it securely. However, even if leaked, an attacker can only access the Shared Drive's backups (which are encrypted with age). They cannot decrypt the backups without the age private key.
- The service account email must remain a member of the Shared Drive with at least Content manager role for both uploads and pruning to work.
