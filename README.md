<p align="center"><img src="assets/logo.png" width="160" alt="FreeTurnX Admin"></p>

# FreeTurnX Admin

Веб-админка для серверов [free-turn-proxy](https://github.com/hackdiaz-dev/free-turn-proxy): подключение к VPS по SSH,
установка и перенастройка сервера, выдача и отзыв доступа пользователям — то, что умеет Android-клиент, но в браузере
и для нескольких серверов сразу.

План — [plan.md](plan.md). Готов этап 1: вход администратора, серверы (SSH-доступ хранится зашифрованным),
журнал действий, SQLite или PostgreSQL, Docker. SSH, установка серверов и пользователи — следующие этапы.

## Запуск

```sh
cp .env.example .env            # задайте POSTGRES_PASSWORD
docker compose up -d --build    # админка + PostgreSQL на http://127.0.0.1:8080
# или одна SQLite:
docker compose -f docker-compose.sqlite.yml up -d --build
```

Без Docker: `task build && FTA_COOKIE_SECURE=false ./build/freeturn-admin`. Разработка: `task dev`, тесты: `task test` и `task test-pg`.

Снаружи открывайте только через HTTPS-прокси (Caddy, nginx) с `FTA_TRUST_PROXY=true`. В томе `/data` лежит мастер-ключ
шифрования секретов — без него пароли и ключи SSH в БД не расшифровать, сохраняйте его вместе с резервной копией БД.

| Переменная | По умолчанию | |
|---|---|---|
| `FTA_LISTEN` | `:8080` | адрес HTTP |
| `FTA_DB_DRIVER` | `sqlite` | `sqlite` или `postgres` |
| `FTA_DB_DSN` | `file:data/freeturn-admin.db` | строка подключения |
| `FTA_DATA_DIR` | `data` | БД SQLite и мастер-ключ |
| `FTA_MASTER_KEY` / `FTA_MASTER_KEY_FILE` | создаётся в `data/master.key` | 32 байта в base64 |
| `FTA_ADMIN_LOGIN` / `FTA_ADMIN_PASSWORD` | — | первый администратор при пустой БД |
| `FTA_COOKIE_SECURE` | `true` | `false` — только для http на localhost |
| `FTA_TRUST_PROXY` | `false` | брать IP клиента из `X-Forwarded-For` |
| `FTA_TLS_CERT` / `FTA_TLS_KEY` | — | TLS без прокси |

Стек: React + Zustand + TanStack Router (фронт), Go + fasthttp (монолит), PostgreSQL или SQLite, Docker Compose.

Клиенты: [macOS](https://github.com/huhhuh78787878-collab/freeturn-macos) · [Windows](https://github.com/huhhuh78787878-collab/freeturn-win).
Новости — в [Telegram-канале](https://t.me/+ZVdbpZB0w_Q1MTdi).

Лицензия — GPL-3.0.
