# FreeTurnX Admin — план

Веб-админка для серверов FreeTurn: то, что Android-клиент [turn-proxy-android](https://github.com/hackdiaz-dev/turn-proxy-android)
умеет делать с VPS по SSH, но в браузере и для нескольких серверов сразу. Клиенты для пользователей —
[freeturn-macos](https://github.com/huhhuh78787878-collab/freeturn-macos) и [freeturn-win](https://github.com/huhhuh78787878-collab/freeturn-win).
Новости — в [Telegram-канале](https://t.me/+ZVdbpZB0w_Q1MTdi).

## 1. Цели

1. Управлять серверами free-turn-proxy из браузера: подключение по SSH, установка, перенастройка, обновление, удаление.
2. Выдавать и отзывать доступ пользователям сервера: ссылка `freeturn://` и QR, как экран «Поделиться» Android-клиента.
3. Несколько серверов в одном месте, с состоянием и журналом.
4. Разворачиваться одной командой: `docker compose up` или один бинарник с SQLite.

## 2. Не цели (первая версия)

- Свой VPN-клиент в браузере — подключаются клиенты для macOS / Windows / Android.
- Биллинг, тарифы, самостоятельная регистрация пользователей VPN.
- Мультиарендность и роли — модель закладываем (поле `role`), но в интерфейсе один уровень «администратор».
- Мониторинг трафика по пользователям сверх того, что отдаёт `client-list` (last handshake).

## 3. Функции по умолчанию (паритет с Android)

Всё делается через `install.sh` из релиза ядра [free-turn-proxy](https://github.com/hackdiaz-dev/free-turn-proxy),
который админка передаёт на сервер по SSH и вызывает как RPC (proto 3, ответ — JSON): `probe`, `apply`, `stop`,
`logs`, `client-list`, `client-add`, `client-conf`, `client-remove`, `uninstall`.

1. **SSH-доступ к серверу**
   - адрес, порт, пользователь; вход по паролю или ключу PEM (в том числе с паролем ключа); пароль sudo, если вход не под root;
   - проверка ключа хоста по TOFU: первый отпечаток запоминается, при смене — отказ и явное предупреждение о возможной подмене;
   - режим root / sudo определяется сам (`detectRootMode`), как в Android;
   - SSH-лог каждой операции: команды без секретов, вывод, коды возврата.
2. **Установка сервера (мастер)**
   - способ: Docker (поставится сам) или systemd;
   - куда идёт трафик: «FreeTurn WG» (свой WireGuard `ft-wg0`: порт, подсеть /16–/29 с проверкой как `valid_net` в install.sh)
     или «Мой VPN» (`host:port` существующего VPN: Amnezia, wg-easy, 3x-ui);
   - порт FreeTurn — случайный из 56000–56999, проверка конфликта с портом WG;
   - режим проброса UDP (WireGuard/AmneziaWG) или TCP (Xray) с профилем ARQ (стандарт / мобильный / свой);
   - обфускация: профиль `rtpopus` / `rtpopus2` / `rtpopus3` / выкл, ключ 64 hex — генерируется, пейсинг 0–60 мс;
   - шаги с прогрессом (проверка SSH → загрузка install.sh → apply → сохранение), повторный запуск безопасен;
   - уже стоит FreeTurn — поля заполняются его настройками (`probe`).
3. **Управление сервером**
   - состояние: установлен / работает / остановлен / недоступен, версия ядра, параметры запуска;
   - применить изменённые настройки (`apply` целиком, а не дельтой — как у Android), обновить до последнего релиза (`--update`),
     остановить, загрузить свою сборку сервера (`--bin`);
   - журнал сервера (`logs --tail`), SSH-лог;
   - удалить FreeTurn с сервера (`uninstall --target=all --purge`) — с подтверждением; удалить запись только из админки.
4. **Пользователи сервера**
   - список из `client-list`: имя, Client ID, WG-адрес, «был в сети N назад» (last handshake), отметка «владелец» (удалять нельзя);
   - выдать доступ: имя (латиница, цифры, `. _ -`), `client-add` + `client-conf` → ссылка `freeturn://` (порядок полей как у Go/Android)
     и QR; опция «вложить ссылку на звонок»; для своего WG — с конфигом пользователя, для «Мой VPN» — только часть FreeTurn;
   - показать ссылку повторно, отозвать доступ (`client-remove`) с подтверждением.
5. **Синхронизация настроек**
   - смена обфускации, пейсинга, режима и ARQ применяется на сервер и попадает в новые ссылки;
   - флаг «перезапускать сервер при смене профиля».
6. **Несколько серверов**: список, добавить, клонировать настройки, переименовать, удалить.

Плюс то, что нужно именно веб-админке:

7. **Вход администратора**: локальные учётки, пароль argon2id, сессии в БД (HttpOnly + Secure + SameSite=Strict cookie),
   CSRF-токен, ограничение попыток входа, первый администратор — из env или мастером при первом запуске.
8. **Хранение секретов**: SSH-пароли, ключи PEM, пароли sudo и ключи обфускации шифруются в БД AES-256-GCM;
   мастер-ключ — из env или файла, в БД его нет.
9. **Журнал действий**: кто, когда, над каким сервером, что сделал, итог.

## 4. Архитектура

Один Go-бинарник (монолит) отдаёт API и собранный фронт (встроен через `embed`).

```
cmd/freeturn-admin       main: конфиг, миграции, запуск HTTP
internal/config          env + yaml, проверка
internal/httpapi         fasthttp-роутер, middleware (сессия, CSRF, rate limit, журнал), JSON
internal/auth            учётки, argon2id, сессии
internal/store           интерфейс хранилища + драйверы postgres (pgx) и sqlite (modernc, без cgo)
internal/store/migrations SQL-миграции, общие для обеих БД (диалектные отличия — в отдельных файлах)
internal/crypto          шифрование секретов (AES-256-GCM, мастер-ключ)
internal/ssh             golang.org/x/crypto/ssh: пароль/ключ, TOFU host key, sudo, стриминг stdin/stdout
internal/server          RPC install.sh: команды proto 3, разбор ответов, загрузка install.sh и бинарника
internal/share           сборка ссылок freeturn:// и QR (порт ShareLinkBuilder)
internal/jobs            фоновые задачи (установка, apply, обновление) с шагами и прогрессом
web/                     фронт: Vite + React + TypeScript + Zustand + TanStack Router
```

- Долгие операции идут задачей: API сразу отвечает `job_id`, прогресс и вывод — по SSE (`/api/jobs/{id}/events`);
  одна операция на сервер одновременно (блокировка в `jobs`).
- install.sh берётся из релиза ядра закреплённой версии (как у Android: `install.sh` из релиза), кэшируется на диске.
- Таймауты как в Android: обычная команда 180 с, `apply` до 15 мин.

## 5. Модель данных

| Таблица | Поля |
|---|---|
| `admins` | id, login, password_hash, role, created_at, last_login_at |
| `sessions` | id (хэш токена), admin_id, csrf, created_at, expires_at, ip, user_agent |
| `servers` | id, name, host, ssh_port, ssh_user, auth_type, ssh_secret_enc, sudo_secret_enc, host_fingerprint, root_mode, created_at |
| `server_settings` | server_id, method (docker/systemd), backend (new/external), connect, wg_port, wg_net, listen_port, proxy_mode, kcp_json, obf_profile, obf_key_enc, obf_timing_ms, provider, transport, threads, streams_per_cred, owner_client_id, sync, restart_on_switch |
| `server_state` | server_id, installed, running, core_version, run_args, checked_at, error |
| `server_users` | server_id, name, client_id, wg_ip, last_handshake, is_owner, created_at — кэш `client-list` |
| `jobs` | id, server_id, kind, status, steps_json, started_at, finished_at, error |
| `job_log` | job_id, seq, at, stream, line |
| `audit` | id, admin_id, server_id, action, details_json, result, at |

## 6. API

Все ответы — JSON; изменяющие запросы требуют сессию и заголовок `X-CSRF-Token`.

```
POST   /api/auth/login            POST /api/auth/logout          GET /api/auth/me
GET    /api/servers               POST /api/servers              GET/PATCH/DELETE /api/servers/{id}
POST   /api/servers/{id}/clone
POST   /api/servers/{id}/ssh/test          проверить вход, отпечаток хоста
POST   /api/servers/{id}/ssh/trust         принять новый отпечаток (после явного подтверждения)
POST   /api/servers/{id}/probe             состояние сервера
POST   /api/servers/{id}/install           → job   (мастер установки)
POST   /api/servers/{id}/apply             → job   (применить настройки)
POST   /api/servers/{id}/update            → job
POST   /api/servers/{id}/upload-binary     → job   (своя сборка)
POST   /api/servers/{id}/stop
POST   /api/servers/{id}/uninstall         → job
GET    /api/servers/{id}/logs?tail=200     журнал сервера
GET    /api/servers/{id}/users             client-list
POST   /api/servers/{id}/users             client-add + client-conf → ссылка и QR
GET    /api/servers/{id}/users/{name}/link ссылка заново (?with_call_link=1)
DELETE /api/servers/{id}/users/{name}      client-remove
GET    /api/jobs/{id}                      GET /api/jobs/{id}/events (SSE)
GET    /api/audit
GET    /api/admins                         POST/DELETE /api/admins (заготовка под роли)
```

## 7. Экраны

1. Вход; первый запуск — создание администратора.
2. Серверы: карточки с состоянием (работает / остановлен / недоступен), версией, числом пользователей.
3. Сервер → «Обзор»: состояние, параметры запуска, кнопки применить / обновить / остановить / удалить.
4. Сервер → «Настройки»: обфускация, режим и ARQ, порты, синхронизация; «Применить» с подтверждением.
5. Сервер → «Пользователи»: таблица, «Выдать доступ» (имя → ссылка и QR, копировать, скачать PNG), «Отозвать».
6. Сервер → «Журналы»: журнал сервера и SSH-лог с поиском.
7. Мастер добавления: SSH (проверка, отпечаток) → параметры → установка с живым прогрессом.
8. Журнал действий, администраторы, настройки админки.

Состояние — Zustand (сессия, текущий сервер, задачи), маршруты — TanStack Router (типизированные параметры
`/servers/$id/users`), запросы — fetch-обёртка с CSRF. Тёмная тема по умолчанию, иконка — `assets/logo.png`.

## 8. Безопасность

- Админка — граница доверия к root-доступу на серверах: только HTTPS снаружи (за обратным прокси или с TLS в конфиге).
- Секреты в БД — только зашифрованными; в логах и SSH-логе пароли, ключи и ссылки на звонок замазываются (как Redactor клиентов).
- TOFU для ключа хоста; смена отпечатка блокирует операции до явного подтверждения.
- Имена пользователей и параметры install.sh валидируются на сервере админки до SSH (белые списки символов, порты, подсеть).
- Rate limit на вход и на API; блокировка учётки после серии неудач; сессии с истечением и отзывом.
- Заголовки: CSP, X-Frame-Options DENY, Referrer-Policy.

## 9. Конфигурация

Env (приоритет) или `config.yaml`:

| Ключ | Значение |
|---|---|
| `FTA_LISTEN` | адрес HTTP, по умолчанию `:8080` |
| `FTA_DB_DRIVER` | `sqlite` (по умолчанию) или `postgres` |
| `FTA_DB_DSN` | `file:/data/freeturn-admin.db` или `postgres://…` |
| `FTA_MASTER_KEY` / `FTA_MASTER_KEY_FILE` | ключ шифрования секретов (32 байта, base64) |
| `FTA_TLS_CERT`, `FTA_TLS_KEY` | TLS без обратного прокси (необязательно) |
| `FTA_ADMIN_LOGIN`, `FTA_ADMIN_PASSWORD` | первый администратор при пустой БД |
| `FTA_CORE_VERSION` | версия ядра, чей install.sh используется |

## 10. Сборка и развёртывание

- Фронт собирается Vite в `web/dist` и встраивается в бинарник (`embed`).
- Бинарники: goreleaser — linux / windows / darwin × amd64 / arm64, `CGO_ENABLED=0` (SQLite через modernc).
- Docker: multi-stage (node → go → distroless), образ для linux/amd64 и linux/arm64.
- `docker-compose.yml`: `admin` + `postgres` (том для данных, healthcheck); вариант `docker-compose.sqlite.yml` — один сервис с томом.
- Taskfile: `task dev` (Vite + go run), `task test`, `task build`, `task release`.

## 11. Тесты

- Go: разбор ответов install.sh (эталоны proto 3), сборка ссылок `freeturn://` (байт в байт с Android/macOS),
  валидация параметров, шифрование секретов, store на обеих БД (postgres — в контейнере), auth и CSRF, TOFU.
- SSH — на тестовом sshd в контейнере с поддельным install.sh: установка, apply, client-add/remove, смена отпечатка.
- Фронт: Vitest для сторов и утилит, Playwright для входа, мастера и выдачи доступа.

## 12. Этапы

1. Каркас: репозиторий, Taskfile, goreleaser, Dockerfile, compose, конфиг, store с миграциями на двух БД, вход администратора.
2. SSH и RPC install.sh: серверы, проверка SSH и TOFU, probe, журнал сервера, SSH-лог.
3. Мастер установки и фоновые задачи с SSE; apply, обновление, остановка, удаление, своя сборка.
4. Пользователи: список, выдача доступа (ссылка + QR), отзыв.
5. Синхронизация настроек, журнал действий, администраторы.
6. Полировка: тесты на живом VPS, документация, релиз.
