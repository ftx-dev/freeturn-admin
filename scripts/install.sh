#!/usr/bin/env bash
# Установщик FreeTurn Admin для Linux (systemd), без Docker:
#   wget -qO- https://github.com/ftx-dev/freeturn-admin/releases/latest/download/install.sh | sudo bash
#   curl -fsSL https://github.com/ftx-dev/freeturn-admin/releases/latest/download/install.sh | sudo bash
# Параметры (после `bash -s --`):
#   --port N            порт веб-интерфейса (по умолчанию 8080)
#   --listen ADDR       адрес целиком, например 127.0.0.1:8080 (за своим HTTPS-прокси)
#   --behind-proxy      админка за HTTPS-прокси: доверять X-Forwarded-*, cookie только по HTTPS
#   --version vX.Y.Z    конкретная версия вместо последней
#   --no-auto-update    не ставить ежедневное автообновление
#   --update            только обновить бинарник, если вышла новая версия (так работает автообновление)
#   --uninstall         удалить службу и бинарник (данные в /var/lib/freeturn-admin остаются)
set -euo pipefail

REPO="ftx-dev/freeturn-admin"
BIN=/usr/local/bin/freeturn-admin
DATA=/var/lib/freeturn-admin
CONF_DIR=/etc/freeturn-admin
ENV_FILE=$CONF_DIR/env
SERVICE=freeturn-admin
USER_NAME=freeturn-admin

PORT=8080 LISTEN="" BEHIND_PROXY=false VERSION="" AUTO_UPDATE=true MODE=install
while [ $# -gt 0 ]; do
  case "$1" in
    --port) PORT="$2"; shift ;;
    --listen) LISTEN="$2"; shift ;;
    --behind-proxy) BEHIND_PROXY=true ;;
    --version) VERSION="$2"; shift ;;
    --no-auto-update) AUTO_UPDATE=false ;;
    --update) MODE=update ;;
    --uninstall) MODE=uninstall ;;
    *) echo "неизвестный параметр: $1" >&2; exit 2 ;;
  esac
  shift
done

say() { printf '\033[1m==>\033[0m %s\n' "$*"; }
die() { printf 'ошибка: %s\n' "$*" >&2; exit 1; }

[ "$(id -u)" -eq 0 ] || die "запустите от root (через sudo)"
[ "$(uname -s)" = Linux ] || die "нужен Linux"
command -v systemctl >/dev/null || die "нужен systemd"

fetch() { # fetch URL [файл] - в файл или в stdout
  if command -v curl >/dev/null; then curl -fsSL "$1" ${2:+-o "$2"}
  elif command -v wget >/dev/null; then wget -q "$1" -O "${2:--}"
  else die "нужен curl или wget"; fi
}

if [ "$MODE" = uninstall ]; then
  systemctl disable --now "$SERVICE" "$SERVICE-update.timer" 2>/dev/null || true
  rm -f /etc/systemd/system/$SERVICE.service /etc/systemd/system/$SERVICE-update.service \
        /etc/systemd/system/$SERVICE-update.timer "$BIN"
  systemctl daemon-reload
  say "удалено; данные и настройки остались в $DATA и $CONF_DIR"
  exit 0
fi

case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  *) die "неподдерживаемая архитектура $(uname -m)" ;;
esac

if [ -z "$VERSION" ]; then
  VERSION="$(fetch "https://api.github.com/repos/$REPO/releases/latest" | sed -n 's/.*"tag_name": *"\([^"]*\)".*/\1/p' | head -1)"
  [ -n "$VERSION" ] || die "не удалось узнать последнюю версию"
fi
VER="${VERSION#v}"

CURRENT="$("$BIN" -version 2>/dev/null || true)"
if [ "$MODE" = update ] && [ "$CURRENT" = "$VER" ]; then
  exit 0
fi

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
ARCHIVE="freeturn-admin_${VER}_linux_${ARCH}.tar.gz"
say "скачиваю FreeTurn Admin $VER ($ARCH)"
fetch "https://github.com/$REPO/releases/download/v$VER/$ARCHIVE" "$TMP/$ARCHIVE"
fetch "https://github.com/$REPO/releases/download/v$VER/SHA256SUMS.txt" "$TMP/SHA256SUMS.txt"
(cd "$TMP" && grep " $ARCHIVE\$" SHA256SUMS.txt | sha256sum -c --quiet -) || die "контрольная сумма не совпала"
tar -xzf "$TMP/$ARCHIVE" -C "$TMP" freeturn-admin
install -m 0755 "$TMP/freeturn-admin" "$BIN.new"
mv -f "$BIN.new" "$BIN"

if [ "$MODE" = update ]; then
  systemctl restart "$SERVICE"
  say "обновлено: ${CURRENT:-?} -> $VER"
  exit 0
fi

id "$USER_NAME" >/dev/null 2>&1 || useradd --system --home-dir "$DATA" --shell /usr/sbin/nologin "$USER_NAME"
install -d -m 0700 -o "$USER_NAME" -g "$USER_NAME" "$DATA"
install -d -m 0750 "$CONF_DIR"

FIRST_RUN=false
PASSWORD=""
if [ ! -f "$ENV_FILE" ]; then
  FIRST_RUN=true
  [ -n "$LISTEN" ] || LISTEN="0.0.0.0:$PORT"
  if $BEHIND_PROXY; then SECURE=true TRUST=true; else SECURE=false TRUST=false; fi
  PASSWORD="$(head -c 32 /dev/urandom | base64 | tr -dc 'A-Za-z0-9' | cut -c1-20)"
  umask 077
  cat > "$ENV_FILE" <<EOF
FTA_DATA_DIR=$DATA
FTA_LISTEN=$LISTEN
FTA_COOKIE_SECURE=$SECURE
FTA_TRUST_PROXY=$TRUST
# Первый администратор: создаётся при пустой базе, после первого запуска строки удаляются.
FTA_ADMIN_LOGIN=admin
FTA_ADMIN_PASSWORD=$PASSWORD
EOF
fi

cat > /etc/systemd/system/$SERVICE.service <<EOF
[Unit]
Description=FreeTurn Admin
After=network-online.target
Wants=network-online.target

[Service]
User=$USER_NAME
Group=$USER_NAME
EnvironmentFile=$ENV_FILE
ExecStart=$BIN
Restart=on-failure
RestartSec=3
NoNewPrivileges=true
ProtectSystem=strict
ProtectHome=true
PrivateTmp=true
ReadWritePaths=$DATA

[Install]
WantedBy=multi-user.target
EOF

if $AUTO_UPDATE; then
  cat > /etc/systemd/system/$SERVICE-update.service <<EOF
[Unit]
Description=FreeTurn Admin: обновление до последнего релиза
After=network-online.target

[Service]
Type=oneshot
ExecStart=/bin/sh -c 'curl -fsSL https://github.com/$REPO/releases/latest/download/install.sh | bash -s -- --update'
EOF
  cat > /etc/systemd/system/$SERVICE-update.timer <<EOF
[Unit]
Description=FreeTurn Admin: ежедневная проверка обновлений

[Timer]
OnCalendar=daily
RandomizedDelaySec=1h
Persistent=true

[Install]
WantedBy=timers.target
EOF
fi

systemctl daemon-reload
systemctl enable --now "$SERVICE" >/dev/null
systemctl restart "$SERVICE"
if $AUTO_UPDATE; then
  command -v curl >/dev/null || say "для автообновления нужен curl: apt install curl"
  systemctl enable --now "$SERVICE-update.timer" >/dev/null
fi

# Ждём, пока служба поднимется и создаст администратора; пароль из файла настроек убираем.
for _ in $(seq 30); do
  systemctl is-active --quiet "$SERVICE" && journalctl -u "$SERVICE" --since "-1min" -q | grep -q 'слушаю' && break
  sleep 1
done
systemctl is-active --quiet "$SERVICE" || { journalctl -u "$SERVICE" -n 20 --no-pager; die "служба не запустилась"; }
if $FIRST_RUN; then
  sed -i '/^# Первый администратор/d; /^FTA_ADMIN_LOGIN=/d; /^FTA_ADMIN_PASSWORD=/d' "$ENV_FILE"
fi

ADDR="$(sed -n 's/^FTA_LISTEN=//p' "$ENV_FILE")"
HOST_PORT="${ADDR##*:}"
IP="$(hostname -I 2>/dev/null | awk '{print $1}')"
say "FreeTurn Admin $VER работает"
echo "    адрес:   http://${IP:-<IP сервера>}:$HOST_PORT  (настройки: $ENV_FILE)"
if $FIRST_RUN && [ -n "$PASSWORD" ]; then
  echo "    логин:   admin"
  echo "    пароль:  $PASSWORD   (сохраните - больше не покажется)"
fi
$AUTO_UPDATE && echo "    автообновление: раз в сутки (systemctl disable --now $SERVICE-update.timer - выключить)"
exit 0
