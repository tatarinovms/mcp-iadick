# mcp-iadick (Яндекс Диск MCP Сервер на Go)

[![Go Version](https://img.shields.io/badge/go-1.27.1+-00ADD8.svg)](https://golang.org)
[![MCP](https://img.shields.io/badge/MCP-Model%20Context%20Protocol-green.svg)](https://modelcontextprotocol.io)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

**mcp-iadick** — быстрый легковесный сервер протокола [Model Context Protocol (MCP)](https://modelcontextprotocol.io/) на **Go**, предназначенный для работы с **Яндекс Диском** через утилиту [rclone](https://rclone.org/).

Сервер собирается в единый бинарный файл, не требует установки внешних интерпретаторов (Python, Node.js) и может работать как **локально (stdio)**, так и **по сети (Streamable HTTP / SSE)** для удаленных клиентов или работы в Docker/Kubernetes/сервере.

---

## Возможности

- **Написан на Go**: единый бинарник. Потребление памяти зависит от размеров файлов, ответов и числа параллельных операций.
- **Сетевой режим**: поддержка **Streamable HTTP (`/mcp`)**, **SSE (`/sse`)** и универсального режима **Dual (`/mcp` + `/sse`)** на одном порту.
- **Навигация**: просмотр содержимого директорий (`yandex_list_directory`), метаданных файлов (`yandex_get_file_info`).
- **Поиск**: поиск файлов по маске/шаблону (`yandex_search_files`).
- **Чтение и запись**: чтение текстовых файлов с лимитом размера (`yandex_read_file`), запись/создание файлов (`yandex_write_file`).
- **Передача бинарных файлов (Base64)**: загрузка любых файлов на Диск из Base64-строки (`yandex_upload_base64`) и скачивание в Base64 (`yandex_download_base64`).
- **Загрузка по ссылке (URL)**: прямое скачивание файлов из интернета по ссылке на Яндекс Диск (`yandex_upload_from_url`).
- **Синхронизация с сервером**: загрузка файлов с локального диска машины на Яндекс Диск (`yandex_upload_file`) и скачивание (`yandex_download_file`).
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

2. **Настроенный пульт `yandex` в rclone со своим OAuth-приложением**:
   Следуйте [инструкции ниже](#настройка-яндекс-диска-обновление-от-24-октября-2026). Обычная авторизация через стандартное приложение rclone больше не даёт доступа к файлам Диска.

3. **Go 1.27.1+** (только при сборке из исходников; точная минимальная версия указана в `go.mod`). CI использует ту же версию из `go.mod`.

---

## Настройка Яндекс Диска: обновление от 24 октября 2026

Яндекс отозвал у стандартного OAuth-приложения rclone права на операции с файлами. Авторизация проходит, но при обращении к Диску появляется ошибка **`410 Gone`**. Обновление до **rclone v1.75** и `rclone config reconnect` не помогают — они выдают токен того же заблокированного приложения.

Для подключения используйте своё OAuth-приложение. Эта настройка нужна как для нового подключения, так и для существующего `yandex`, если оно использует токен стандартного приложения rclone.

### 1. Создаём своё приложение

Откройте [Яндекс OAuth](https://oauth.yandex.ru/) и нажмите **Создать → Для доступа к API или отладки**.

В разделе **Доступ к данным** выдайте приложению три права:

- чтение всего Диска;
- запись в любом месте Диска;
- доступ к информации о Диске.

Создайте приложение и скопируйте его **ClientID**.

### 2. Получаем свой токен

Откройте в браузере ссылку, подставив скопированный ClientID вместо `<ClientID>`:

```text
https://oauth.yandex.ru/authorize?response_type=token&client_id=<ClientID>
```

Нажмите **Продолжить → Войти вашим аккаунтом Яндекса** и скопируйте выданную строку. Это токен — **никому его не показывайте**.

### 3. Правим конфиг

Узнайте путь к конфигу:

```bash
rclone config file
```

Откройте файл в любом редакторе. Если подключение `yandex` уже есть, в секции `[yandex]` замените строку `token`. Если настраиваете всё с нуля — добавьте секцию целиком:

```ini
[yandex]
type = yandex
token = {"access_token":"***","token_type":"OAuth","refresh_token":"none","expiry":"2027-10-01T00:00:00+03:00"}
```

Вместо `***` вставьте полученный токен. Значение `refresh_token` оставьте равным `none`.

Сохраните конфиг и проверьте подключение:

```bash
rclone lsd yandex:
```

Подробнее — в [статье «rclone для Яндекс Диска»](https://blog.tatarinovms.space/rclone4yd/).

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
- Запускается автоматически при создании и отправке любого тега вида `v*` (например, `v0.2.0`):
  ```bash
  git tag v0.2.0
  git push origin v0.2.0
  ```
- Автоматически компилирует статические бинарники под 7 целевых архитектур (Linux amd64/arm64/armv7, macOS amd64/arm64, Windows amd64/arm64).
- Перед сборкой запускает `go test -race -cover ./...` и `go vet ./...`; при ошибках релиз не собирается. Те же проверки выполняются при push в ветку и в pull request (`.github/workflows/test.yml`).
- Упаковывает архивы (`.tar.gz` / `.zip`), вычисляет контрольные суммы SHA256 (`checksums.txt`) и публикует релиз на GitHub.

---

## Режимы работы

### 1. Локальный режим (stdio)
Используется по умолчанию для локальных MCP клиентов (OpenCode v2, Claude Code, Hermes и др.):
```bash
./bin/mcp-iadick -transport stdio
```

### 2. Режим Streamable HTTP (рекомендуется для OpenCode v2 и Hermes)
Современный протокол спецификации MCP. OpenCode v2 и Hermes для удаленных серверов отправляют запросы `POST /mcp`:
```bash
./bin/mcp-iadick -transport http -addr :8080
```
Endpoint для подключения: `http://<host>:8080/mcp`

### 3. Режим SSE (Server-Sent Events)
Сетевой протокол SSE (`GET /sse` + `POST /message?sessionId=...`), поддерживаемый сетевыми клиентами (включая Claude Code):
```bash
./bin/mcp-iadick -transport sse -addr :8080 -base-url http://<host>:8080
```
Endpoint для подключения: `http://<host>:8080/sse`

### 4. Универсальный сетевой режим Dual (Streamable HTTP + SSE)
Одновременно поднимает **и `/mcp`** (Streamable HTTP для OpenCode v2 / Hermes), **и `/sse`** (для Claude Code / SSE клиентов) на одном порту:
```bash
./bin/mcp-iadick -transport dual -addr :8080
```
- OpenCode v2 / Hermes подключаются к `http://<host>:8080/mcp`
- Claude Code и SSE-клиенты подключаются к `http://<host>:8080/sse`

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

> Важно: в URL обязательно должен быть суффикс `/mcp` (Streamable HTTP).

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

### 2. Claude Code

#### Сетевое подключение (SSE):
Через CLI Claude Code:
```bash
claude mcp add --transport sse yandex-disk http://<SERVER_IP>:8080/sse
```

#### Локальный запуск (stdio):
Через CLI Claude Code:
```bash
claude mcp add yandex-disk -- /path/to/bin/mcp-iadick
```

Или через конфигурационный файл `~/.claude.json` / `.claude.json`:
```json
{
  "mcpServers": {
    "yandex-disk": {
      "command": "/path/to/bin/mcp-iadick",
      "env": {
        "RCLONE_REMOTE": "yandex"
      }
    }
  }
}
```

---

### 3. Hermes Agent (`~/.hermes/config.yaml`)

#### Сетевое подключение к серверу:
В `~/.hermes/config.yaml`:
```yaml
mcp_servers:
  yandex-disk:
    url: "http://<SERVER_IP>:8080/mcp"
```

Через CLI Hermes:
```bash
hermes mcp add yandex-disk --url http://<SERVER_IP>:8080/mcp
```

#### Локальный запуск бинарника (stdio):
```yaml
mcp_servers:
  yandex-disk:
    command: "/path/to/bin/mcp-iadick"
    env:
      RCLONE_REMOTE: "yandex"
```

Через CLI Hermes:
```bash
hermes mcp add yandex-disk --command "/path/to/bin/mcp-iadick"
```

Проверка подключения в сессии Hermes:
```
/reload-mcp
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
| `-operation-timeout` | Предельное время операции, включая ожидание свободного процесса; положительная длительность Go, например `30s` или `30m` | `5m` |
| `-max-concurrent` | Максимальное число одновременно работающих процессов rclone; положительное число | `4` |
| `-log-operations` | Писать в stderr название команды rclone, длительность и результат; пути и содержимое не включаются | `false` |
| `-version` | Показать версию бинарника и выйти | `false` |

Для длительных передач увеличьте таймаут, например:

```bash
./bin/mcp-iadick -transport http -operation-timeout 30m -max-concurrent 2 -log-operations
```

### Ограничения ресурсов

- `max_bytes` для текстового чтения и Base64 должен быть от 1 до **10 МиБ** (10 485 760 байт). Значения 0 и отрицательные больше не означают чтение без ограничения. `offset` должен быть неотрицательным.
- Текстовая запись и загрузка через Base64 ограничены **10 МиБ** содержимого. Входная Base64-строка ограничена 13 985 112 байтами, включая префикс и пробельные символы. Для больших файлов используйте `yandex_upload_file`, `yandex_download_file` или `yandex_upload_from_url`: ограничение содержимого в 10 МиБ к ним не применяется.
- HTTP, SSE и Dual принимают тела запросов размером до **16 МиБ**, включая JSON. Вывод одного процесса rclone ограничен **16 МиБ**, диагностика stderr — **64 КиБ**. При превышении stdout операция завершается с ошибкой. Эти пределы не ограничивают объём файлов, передаваемых командами `copyto` и `copyurl`.
- Списки каталогов и поиск разбираются из потока rclone. Сервер хранит выбранную страницу, а после чтения одного дополнительного элемента останавливает обход.

### Страницы списков и поиска

`yandex_list_directory` и `yandex_search_files` принимают `limit` (по умолчанию 1000, максимум 5000) и `offset` (по умолчанию 0). Ответ содержит `count` — число элементов **на странице**, `offset`, `has_more` и, если есть продолжение, `next_offset`. Пустой список возвращается как `items: []`.

Каждая страница заново запускает обход и пропускает первые `offset` элементов. Это не снимок каталога: порядок зависит от rclone, поэтому между запросами возможны пропуски и повторы, даже если файлы не менялись. Для точного полного списка используйте достаточно большой `limit` в пределах 5000 и сужайте `path`, `max_depth` или шаблон; для очень больших деревьев обходите подпапки отдельно. Пропускаемые элементы также учитываются в пределе вывода rclone.

Необязательный `fields` выбирает поля ответа, например `"Path,Size"`. Доступны `Path`, `Name`, `Size`, `SizeHuman`, `MimeType`, `ModTime`, `IsDir`; без `fields` возвращаются все обычные поля.

При `recursive=true` положительный `max_depth` действительно ограничивает глубину обхода; 0 или отсутствие параметра означает неограниченную глубину. Без рекурсии возвращаются непосредственные дочерние элементы. Параметры `dirs_only` и `files_only` нельзя включать одновременно. Поиск возвращает только файлы.

Пример аргументов для первой страницы:

```json
{"path":"Documents","recursive":true,"max_depth":2,"limit":100,"offset":0,"fields":"Path,Size"}
```

---

## Список инструментов (MCP Tools)

| Инструмент | Аргументы | Описание |
|---|---|---|
| `yandex_list_directory` | `path`, `recursive`, `max_depth`, `dirs_only`, `files_only`, `limit`, `offset`, `fields` | Список файлов и папок с размерами и датами, по страницам |
| `yandex_get_file_info` | `path` | Детальная информация и метаданные пути |
| `yandex_read_file` | `path`, `max_bytes`, `offset` | Чтение содержимого текстового файла |
| `yandex_write_file` | `path`, `content` | Запись текста в файл на Яндекс Диске |
| `yandex_upload_base64` | `remote_path`, `content_base64` | Загрузка файлов (включая бинарные: изображения, документы, архивы) из Base64-строки напрямую на Диск |
| `yandex_download_base64` | `path`, `max_bytes`, `offset` | Скачивание/чтение файла с Диска в кодировке Base64 (для бинарных файлов) |
| `yandex_upload_from_url` | `url`, `remote_path`, `auto_filename` | Прямое скачивание файла по ссылке (URL) из интернета на Диск (без промежуточного сохранения) |
| `yandex_create_directory`| `path` | Создание новой папки |
| `yandex_delete_file` | `path` | Удаление отдельного файла |
| `yandex_delete_directory`| `path`, `recursive` | Удаление каталога (с `recursive=True` удаляет все внутри) |
| `yandex_copy_item` | `source_path`, `destination_path` | Копирование файла/папки внутри Диска |
| `yandex_move_item` | `source_path`, `destination_path` | Перемещение/переименование файла или папки |
| `yandex_upload_file` | `local_path`, `remote_path` | Загрузка файла с локального диска сервера на Диск |
| `yandex_download_file` | `remote_path`, `local_path` | Скачивание файла с Диска на локальный диск сервера |
| `yandex_search_files` | `pattern`, `path`, `max_depth`, `limit`, `offset`, `fields` | Рекурсивный поиск файлов по маске (например, `*.pdf`), по страницам |
| `yandex_get_storage_info`| - | Получение квоты (всего, занято, свободно, %) |
| `yandex_create_public_link` | `path`, `expire` | Создание публичной ссылки `yadi.sk` на файл или папку |
| `yandex_remove_public_link` | `path` | Закрытие общего доступа (отзыв ссылки) |

---

## Готовые скиллы (Agent Skills)

В репозитории подготовлены навыки по спецификации **Agent Skills** (размещены в каталогах `.agents/skills/` и `.opencode/skills/`), которые автоматически распознаются AI-агентами (OpenCode v2, Claude Code, Hermes Agent, Antigravity):

1. **`yandex-disk`** (`Yandex Disk Management`): навигация, поиск, чтение и запись текстовых файлов, создание папок, управление публичными ссылками, проверка квоты.
2. **`yandex-disk-transfer`** (`Yandex Disk File Transfer & Import`): передача бинарных файлов (изображения, архивы, PDF) через Base64, прямое скачивание по внешним HTTP/HTTPS URL, работа с файлами хоста.
3. **`yandex-disk-backup`** (`Yandex Disk Backup & Snapshot`): создание структурированных резервных копий по расписанию/датам, проверка дискового пространства, генерация временных публичных ссылок на скачивание, ротация бэкапов.

---

## Тестирование

Запуск тестов:
```bash
go test -race -cover ./...
go vet ./...
```

Тесты процессов используют отдельный запуск тестового бинарника вместо rclone и не обращаются к Яндекс Диску. Если rclone установлен, дополнительный тест проверяет глубину обхода и страницы на временном локальном каталоге.

Для сравнения потокового чтения страницы из 100 элементов с прежним разбором всего списка из 10 000 элементов:

```bash
go test ./internal/rclone -run '^$' -bench BenchmarkDecodeListPage -benchmem
```

Это синтетический замер обработки JSON; он не измеряет задержки Яндекс Диска и запуск rclone. Для наблюдения за длительностью реальных операций используйте `-log-operations`.

---

## Лицензия

MIT License (см. [LICENSE](LICENSE)).
