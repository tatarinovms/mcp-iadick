---
name: Yandex Disk Backup & Snapshot
description: Create backups, save repository/project snapshots, export logs or database dumps to Yandex Disk, and generate expiring shareable links. Use when creating backups, archiving artifacts, or sharing build outputs.
---

# Yandex Disk Backup & Snapshot Skill

This skill guides the AI agent through creating structured, timestamped backups of repositories, configs, databases, and assets to Yandex Disk, managing retention, and creating public download links.

## Backup Lifecycle & Procedure

### Step 1: Pre-flight Quota Check
Always verify available cloud space before starting a backup operation:
1. Call `yandex_get_storage_info()`.
2. Inspect `free_bytes`, `used_percentage`, and `free_human`.
3. If free space is below the expected backup size, notify the user immediately and do not proceed.

### Step 2: Directory Naming & Organization
Organize backups hierarchically by project and ISO 8601 timestamp:
- Format: `Backups/<project-name>/YYYY-MM-DD_HHMM/` or `Backups/<project-name>/<version>/`
- Example: `Backups/mcp-iadick/2026-10-08_backup.zip`

1. Create target folder:
   ```json
   yandex_create_directory(path: "Backups/my-project/2026-10-08")
   ```

### Step 3: Archive & Upload
Choose the transfer method based on environment:

- **Local Machine / Host Server**:
  If the file is already on the server's local disk:
  ```json
  yandex_upload_file(
    local_path: "/var/backups/dump.tar.gz",
    remote_path: "Backups/my-project/2026-10-08/dump.tar.gz"
  )
  ```
- **Remote Web Artifacts** (e.g. CI/CD build outputs, S3, release assets):
  Stream directly to Yandex Disk:
  ```json
  yandex_upload_from_url(
    url: "https://github.com/org/repo/releases/download/v1.0.0/build.zip",
    remote_path: "Backups/my-project/v1.0.0/build.zip"
  )
  ```
- **Small configs / archives via MCP**:
  Transfer encoded in Base64:
  ```json
  yandex_upload_base64(
    remote_path: "Backups/my-project/2026-10-08/config.json",
    content_base64: "..."
  )
  ```

### Step 4: Verification & Integrity
Verify that the uploaded item exists and has a non-zero size:
1. Call `yandex_get_file_info(path: "Backups/my-project/2026-10-08/dump.tar.gz")`.
2. Check `SizeHuman` and `ModTime`.

### Step 5: Generating Sharing / Recovery Link (Optional)
If the user requests a download link for team members or recovery:
1. Call `yandex_create_public_link(path: "Backups/my-project/2026-10-08/dump.tar.gz", expire: "7d")`.
2. Return the public link `yadi.sk` with its expiration time.
3. If needed later, revoke access using `yandex_remove_public_link(path: "...")`.

### Step 6: Backup Rotation / Retention (Optional)
To avoid running out of quota:
1. List old backups: `yandex_list_directory(path: "Backups/my-project", dirs_only: true)`.
2. Identify backups older than retention policy (e.g. > 30 days).
3. Confirm with the user before deleting older directories:
   `yandex_delete_directory(path: "Backups/my-project/old-date", recursive: true)`.
