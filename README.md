# mcp-iadick (Яндекс Диск MCP Сервер на Go)

[![Go Version](https://img.shields.io/badge/go-1.22+-00ADD8.svg)](https://golang.org)
[![MCP](https://img.shields.io/badge/MCP-Model%20Context%20Protocol-green.svg)](https://modelcontextprotocol.io)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**mcp-iadick** — быстрый легковесный сервер протокола [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) на **Go**, предназначенный для работы с **Яндекс Диском** через утилиту [rclone](https://rclone.org/).

Сервер собирается в единый бинарный файл, не требует установки внешних интерпретаторов (Python, Node.js) и может работать как **локально (stdio)**, так и **по сети (Streamable HTTP / SSE)** для удаленных клиентов или работы в Docker/Kubernetes/сервере.

---

## Возможности

- **Написан на Go**: быстрый старт, низкое потребление памяти (< 15 МБ RAM), единый бинарник.
- **Сетевой режим**: поддержка **Streamable HTTP (`/mcp`)**, **SSE (`/sse`)** и универсального режима **Dual (`/mcp` + `/sse`)** на одном порту.
- **Навигация**: просмотр содержимого директорий (`yandex_list_directory`), метаданных файлов (`yandex_get_file_info`).
- **Поиск**: поиск файлов по маске/шаблону (`yandex_search_files`).
- **Чтение и запись**: чтение текстовых файлов с лимитом размера (`yandex_read_file`), запись/создание файлов (`yandex_write_file`).
- **Синхронизация**: загрузка файлов с локального диска на Яндекс Диск (`yandex_upload_file`) и скачивание (`yandex_download_file`).
- **Управление**: создание папок (`yandex_create_directory`), удаление файлов (`yandex_delete_file`) и каталогов (`yandex_delete_directory`).
- **Перемещение и копирование**: `yandex_move_item`, `yandex_copy_item`.
- **Квота диска**: получение информации о свободном, занятом и общем месте (`yandex_get_storage_info`).
- **Публичные ссылки**: создание (`yandex_create_public_link`) и отзыв (`yandex_remove_public_link`) ссылок вида `yadi.sk`.

---

## Предварительные требования

1. **Установленный `rclone`**:
   - macOS: `brew install rclone`
   - Linux: `sudo apt install rclone` или `curl https://rclone.org/install.sh | sudo bash`
   - Windows: `winget install Rclone.Rclone`

2. **Настроенный пульт `yandex` в rclone**:
   Если еще не настроен:
   ```bash
   rclone config
   # Выберите n (New remote), имя: yandex, тип: yandex (номер из списка)
   # Пройдите авторизацию через браузер
   ```
   Проверьте работу:
   ```bash
   rclone lsd yandex:
   ```

3. **Go 1.22+** (только при сборке из исходников).

---

## Установка и сборка

### Вариант 1. Скачивание готового бинарника из Releases

Готовые скомпилированные бинарники для всех популярных платформ доступны на странице [GitHub Releases](https://github.com/tatarinovms/mcp-iadick/releases):
- **Linux**: `amd64`, `arm64`, `armv7`
- **macOS**: `arm64` (Apple Silicon M1/M2/M3/M4), `amd64` (Intel)
- **Windows**: `amd64`, `arm64`

### Вариант 2. Сборка из исходников

Соберите бинарный файл для текущей системы:
```bash
go build -o bin/mcp-iadick ./cmd/mcp-iadick
```

Кросс-компиляция (например, для Linux ARM64):
```bash
GOOS=linux GOARCH=arm64 go build -o bin/mcp-iadick-linux-arm64 ./cmd/mcp-iadick
```

---

## Автоматическая сборка релизов (CI/CD)

В репозитории настроен GitHub Actions workflow (`.github/workflows/release.yml`):
- Запускается автоматически при создании и отправке любого тега вида `v*` (например, `v0.1.0`):
  ```bash
  git tag v0.1.0
  git push origin v0.1.0
  ```
- Автоматически компилирует статические бинарники под 7 целевых архитектур (Linux amd64/arm64/armv7, macOS amd64/arm64, Windows amd64/arm64).
- Упаковывает архивы (`.tar.gz` / `.zip`), вычисляет контрольные суммы SHA256 (`checksums.txt`) и публикует релиз на GitHub.

---

## Режимы работы

### 1. Локальный режим (stdio)
Используется по умолчанию для локальных MCP клиентов (Claude Desktop, Cursor и др.):
```bash
./bin/mcp-iadick -transport stdio
```

### 2. Режим Streamable HTTP (рекомендуется для OpenCode v2)
Современный протокол спецификации MCP. OpenCode v2 для удаленных серверов отправляет запросы `POST /mcp`:
```bash
./bin/mcp-iadick -transport http -addr :8080
```
Endpoint для подключения: `http://<host>:8080/mcp`

### 3. Режим SSE (Server-Sent Events)
Legacy-протокол (`GET /sse` + `POST /message?sessionId=...`), используемый некоторыми клиентами (Claude Desktop):
```bash
./bin/mcp-iadick -transport sse -addr :8080 -base-url http://<host>:8080
```
Endpoint для подключения: `http://<host>:8080/sse`

### 4. Универсальный сетевой режим Dual (Streamable HTTP + SSE)
Одновременно поднимает **и `/mcp`** (для OpenCode v2), **и `/sse`** (для Claude Desktop/legacy) на одном порту:
```bash
./bin/mcp-iadick -transport dual -addr :8080
```
- OpenCode v2 подключается к `http://<host>:8080/mcp`
- Claude Desktop подключается к `http://<host>:8080/sse`

---

## Настройка MCP клиентов

### 1. OpenCode v2 (`opencode.json` / `opencode.jsonc`)

#### Сетевое подключение к серверу:
В `~/.config/opencode/opencode.jsonc` (или локальном `opencode.json`):
```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "servers": {
      "yandex-disk": {
        "type": "remote",
        "url": "http://<SERVER_IP>:8080/mcp"
      }
    }
  }
}
```

> Важно: в URL обязательно должен быть суффикс `/mcp` (Streamable HTTP). Если OpenCode ранее кешировал статус ошибки, перезапустите фоновый сервис OpenCode или сессию.

Через CLI OpenCode:
```bash
opencode mcp add yandex-disk --url http://<SERVER_IP>:8080/mcp
```

Проверка статуса подключения:
```bash
opencode mcp list
```

#### Локальный запуск бинарника (stdio):
```jsonc
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "servers": {
      "yandex-disk": {
        "type": "local",
        "command": ["/path/to/bin/mcp-iadick"],
        "environment": {
          "RCLONE_REMOTE": "yandex"
        }
      }
    }
  }
}
```

---

### 2. Claude Desktop (`claude_desktop_config.json`)

#### Сетевое подключение (SSE):
```json
{
  "mcpServers": {
    "yandex-disk": {
      "type": "sse",
      "url": "http://<SERVER_IP>:8080/sse"
    }
  }
}
```

#### Локальный запуск (stdio):
```json
{
  "mcpServers": {
    "yandex-disk": {
      "command": "/path/to/bin/mcp-iadick",
      "args": ["-transport", "stdio"],
      "env": {
        "RCLONE_REMOTE": "yandex"
      }
    }
  }
}
```

---

### 3. Cursor / Antigravity / Cline

В сетевом режиме: `http://<SERVER_IP>:8080/mcp` или `http://<SERVER_IP>:8080/sse`.

Для локального запуска:
```json
{
  "mcpServers": {
    "yandex-disk": {
      "command": "/path/to/bin/mcp-iadick"
    }
  }
}
```

---

## Флаги командной строки и переменные окружения

| Флаг CLI | Описание | По умолчанию |
|---|---|---|
| `-transport` | Протокол транспорта: `stdio`, `http` (Streamable HTTP), `sse`, `dual` (HTTP + SSE) | `stdio` |
| `-addr` | Сетевой адрес для сетевых режимов (например, `:8080`, `0.0.0.0:8080`) | `:8080` |
| `-base-url` | Базовый URL для SSE сервера | `http://localhost:<port>` |
| `-remote` | Имя пульта rclone (или переменная `RCLONE_REMOTE`) | `yandex` |
| `-rclone-path` | Путь к бинарнику rclone (или переменная `RCLONE_PATH`) | автопоиск |
| `-version` | Показать версию бинарника и выйти | `false` |

---

## Список инструментов (MCP Tools)

| Инструмент | Аргументы | Описание |
|---|---|---|
| `yandex_list_directory` | `path`, `recursive`, `max_depth`, `dirs_only`, `files_only` | Список файлов и папок с размерами и датами |
| `yandex_get_file_info` | `path` | Детальная информация и метаданные пути |
| `yandex_read_file` | `path`, `max_bytes`, `offset` | Чтение содержимого текстового файла |
| `yandex_write_file` | `path`, `content` | Запись текста в файл на Яндекс Диске |
| `yandex_create_directory`| `path` | Создание новой папки |
| `yandex_delete_file` | `path` | Удаление отдельного файла |
| `yandex_delete_directory`| `path`, `recursive` | Удаление каталога (с `recursive=True` удаляет все внутри) |
| `yandex_copy_item` | `source_path`, `destination_path` | Копирование файла/папки внутри Диска |
| `yandex_move_item` | `source_path`, `destination_path` | Перемещение/переименование файла или папки |
| `yandex_upload_file` | `local_path`, `remote_path` | Загрузка файла с локального компьютера на Диск |
| `yandex_download_file` | `remote_path`, `local_path` | Скачивание файла с Диска на локальный компьютер |
| `yandex_search_files` | `pattern`, `path` | Рекурсивный поиск файлов по маске (например, `*.pdf`) |
| `yandex_get_storage_info`| - | Получение квоты (всего, занято, свободно, %) |
| `yandex_create_public_link` | `path`, `expire` | Создание публичной ссылки `yadi.sk` на файл или папку |
| `yandex_remove_public_link` | `path` | Закрытие общего доступа (отзыв ссылки) |

---

## Тестирование

Запуск тестов:
```bash
go test -v ./...
```

---

## Лицензия

MIT License (см. [LICENSE](LICENSE)).
