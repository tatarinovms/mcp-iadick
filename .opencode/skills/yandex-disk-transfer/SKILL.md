---
name: Yandex Disk File Transfer & Import
description: Upload, download, and import binary files, images, documents, and web links to/from Yandex Disk. Use when transferring binary files via Base64, downloading directly from external URLs, or synchronizing server files.
---

# Yandex Disk File Transfer & Import Skill

This skill guides the AI agent when transferring binary or remote files between the client/local environment, external web sources, and Yandex Disk using `mcp-iadick`.

## Choosing the Right Transfer Tool

| Source / Target | Tool to Use | Why |
|---|---|---|
| **Binary file from client/chat** (images, PDF, zip) | `yandex_upload_base64` | Encodes binary bytes safely over MCP JSON-RPC protocol. Direct cloud upload via `rclone rcat`. |
| **Binary file to client/chat** | `yandex_download_base64` | Reads bytes from Disk and returns a Base64-encoded string for the client to save locally. |
| **External web link** (HTTP/HTTPS URL) | `yandex_upload_from_url` | Direct server-to-cloud stream via `rclone copyurl`. Bypasses local client disk entirely. |
| **File on the MCP host machine** | `yandex_upload_file` | Transfers directly from host filesystem path to Yandex Disk via `rclone copyto`. |
| **File to the MCP host machine** | `yandex_download_file` | Saves file from Yandex Disk to a path on the host server. |

---

## Detailed Workflows

### 1. Uploading Binary Files (Base64)
Use `yandex_upload_base64(remote_path, content_base64)`:
- `remote_path`: Path on Yandex Disk, e.g. `Images/banner.png` or `Documents/contract.pdf`.
- `content_base64`: The Base64 string.
  - Standard base64 strings or Data URL format (`data:image/png;base64,iVBOR...`) are both accepted.
  - Cleaned automatically from whitespaces and linebreaks.

**Example Steps:**
1. If the target folder doesn't exist, create it: `yandex_create_directory(path: "Images")`.
2. Convert file bytes to base64.
3. Call `yandex_upload_base64(remote_path: "Images/banner.png", content_base64: "...")`.
4. Verify upload with `yandex_get_file_info(path: "Images/banner.png")`.

---

### 2. Downloading Binary Files (Base64)
Use `yandex_download_base64(path, max_bytes, offset)`:
- `path`: Target file on Yandex Disk.
- `max_bytes`: Maximum bytes to retrieve (default: 10 MB = `10485760`).
- `offset`: Starting byte offset (for chunked downloads if needed).

**Example Steps:**
1. Check file size first via `yandex_get_file_info(path: "Archive/data.zip")`.
2. If size is within limits (< 10 MB), retrieve `yandex_download_base64(path: "Archive/data.zip")`.
3. Decode base64 and write the resulting file to the destination.

---

### 3. Importing Files from External URLs
Use `yandex_upload_from_url(url, remote_path, auto_filename)`:
- `url`: Direct HTTP or HTTPS link to the file.
- `remote_path`:
  - If target is a folder (e.g. `Downloads/` or ends with `/`), set `auto_filename: true`. Rclone will extract the file name from the URL or headers.
  - If target specifies exact filename (e.g. `Downloads/linux-kernel.tar.gz`), it will be saved with that exact name.

**Example Steps:**
1. Call `yandex_upload_from_url(url: "https://example.com/assets/logo.svg", remote_path: "Assets/logo.svg")`.
2. Check result status: `"uploaded_from_url"` and confirmed filename.

---

### 4. Working with Local Files on MCP Host
Use `yandex_upload_file(local_path, remote_path)`:
- Note: `local_path` must be accessible on the filesystem where `mcp-iadick` is running.
- Ensure destination path is unambiguous.
