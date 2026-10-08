---
name: Yandex Disk Management
description: Browse, search, read, write, organize files, manage folders, check quota, and generate shareable links on Yandex Disk using mcp-iadick. Use whenever the user asks to inspect or manipulate cloud files on Yandex Disk.
---

# Yandex Disk Management Skill

This skill guides the AI agent in interacting with Yandex Disk cloud storage using the `mcp-iadick` MCP tools.

## Key Tools Overview

| Tool | Purpose | Key Parameters |
|---|---|---|
| `yandex_list_directory` | List folder contents | `path`, `recursive`, `max_depth`, `dirs_only`, `files_only` |
| `yandex_get_file_info` | Get file metadata & stats | `path` |
| `yandex_search_files` | Search by pattern | `pattern` (e.g. `*.pdf`), `path` |
| `yandex_read_file` | Read text file content | `path`, `max_bytes`, `offset` |
| `yandex_write_file` | Create or overwrite text file | `path`, `content` |
| `yandex_create_directory` | Create a folder | `path` |
| `yandex_delete_file` | Delete a single file | `path` |
| `yandex_delete_directory` | Delete a folder | `path`, `recursive` |
| `yandex_copy_item` | Copy within Yandex Disk | `source_path`, `destination_path` |
| `yandex_move_item` | Move or rename file/folder | `source_path`, `destination_path` |
| `yandex_get_storage_info` | Check total, used, free quota | (none) |
| `yandex_create_public_link` | Create public `yadi.sk` link | `path`, `expire` (e.g. `'7d'`) |
| `yandex_remove_public_link` | Revoke public link | `path` |

---

## Rules & Best Practices

### 1. Path Conventions
- Root directory is represented by an empty string `""` or `"/"`.
- Subfolders should omit leading slashes for clarity (e.g., `Documents/Reports`, not `/Documents/Reports/`).
- Avoid backslashes; always use forward slashes `/`.

### 2. Browsing & Exploration
- For initial exploration, list the root or target folder with `max_depth: 1` to prevent huge payloads.
- Use `yandex_search_files` with wildcard patterns (e.g. `*2026*`, `*.xlsx`) when looking for specific documents rather than recursive listing.
- Before large operations, call `yandex_get_storage_info` to verify remaining storage quota.

### 3. Reading and Writing Content
- **Text files**: Use `yandex_read_file` and `yandex_write_file`.
- **Large text files**: Use `max_bytes` and `offset` to paginate content without exhausting token limits.
- **Binary files** (images, archives, PDFs): Do NOT use `yandex_read_file` or `yandex_write_file`! Use `yandex_download_base64` and `yandex_upload_base64` instead.

### 4. Safety Considerations
- **Deletion**: Before deleting a folder with `recursive: true`, confirm contents using `yandex_list_directory`. Warn the user if destructive data loss is possible.
- **Moving/Renaming**: `yandex_move_item` requires both the full `source_path` and target `destination_path` including the target file name.

---

## Common Workflows

### 1. Exploring Disk Contents
```markdown
1. Call `yandex_get_storage_info` to see total space and usage.
2. Call `yandex_list_directory` with `path: ""` and `max_depth: 1` to list root directories.
3. Drill down into specific folders as requested.
```

### 2. Searching & Retrieving Notes
```markdown
1. Search for files matching pattern: `yandex_search_files(pattern: "*.md", path: "Notes")`.
2. Retrieve metadata using `yandex_get_file_info(path: "Notes/meeting.md")`.
3. Read content using `yandex_read_file(path: "Notes/meeting.md", max_bytes: 50000)`.
```

### 3. Creating & Sharing Documents
```markdown
1. Ensure parent folder exists or create it with `yandex_create_directory(path: "Shared")`.
2. Write content: `yandex_write_file(path: "Shared/readme.txt", content: "...")`.
3. Generate share link: `yandex_create_public_link(path: "Shared/readme.txt", expire: "30d")`.
4. Provide the public link `yadi.sk` to the user.
```
