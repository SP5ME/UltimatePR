#!/bin/sh
set -eu

[ "$(id -u)" -eq 0 ] || { echo "Uruchom instalator jako root." >&2; exit 1; }
SOURCE_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
[ -x "$SOURCE_DIR/ultimatepr" ] || { echo "Brak pliku ultimatepr obok instalatora." >&2; exit 1; }
[ -f "$SOURCE_DIR/ultimatepr-update" ] || { echo "Brak pliku ultimatepr-update obok instalatora." >&2; exit 1; }

ensure_group() {
  if getent group ultimatepr >/dev/null 2>&1; then
    return 0
  fi
  if command -v groupadd >/dev/null 2>&1; then
    groupadd --system ultimatepr && return 0
  fi
  if command -v addgroup >/dev/null 2>&1; then
    if addgroup --help 2>&1 | grep -q -- '--system'; then
      addgroup --system ultimatepr && return 0
    fi
    addgroup -S ultimatepr && return 0
  fi
  echo "Nie udało się utworzyć grupy ultimatepr." >&2
  exit 1
}

ensure_user() {
  if id ultimatepr >/dev/null 2>&1; then
    return 0
  fi
  if command -v useradd >/dev/null 2>&1; then
    useradd --system --no-create-home --shell /usr/sbin/nologin --gid ultimatepr ultimatepr && return 0
  fi
  if command -v adduser >/dev/null 2>&1; then
    if adduser --help 2>&1 | grep -q -- '--system'; then
      adduser --system --no-create-home --shell /usr/sbin/nologin --ingroup ultimatepr ultimatepr && return 0
    fi
    adduser -S -D -H -s /sbin/nologin -G ultimatepr ultimatepr && return 0
  fi
  echo "Nie udało się utworzyć użytkownika ultimatepr." >&2
  exit 1
}

ensure_group
ensure_user
if ! id -Gn ultimatepr 2>/dev/null | tr ' ' '\n' | grep -qx ultimatepr; then
  if command -v usermod >/dev/null 2>&1; then
    usermod -a -G ultimatepr ultimatepr
  elif command -v addgroup >/dev/null 2>&1; then
    addgroup ultimatepr ultimatepr
  fi
fi
install -d -m 0755 /opt/ultimatepr /etc/ultimatepr /usr/local /usr/local/sbin
install -d -m 0750 -o ultimatepr -g ultimatepr /var/lib/ultimatepr /var/lib/ultimatepr/backups
touch /var/log/ultimatepr.log
chown ultimatepr:ultimatepr /var/log/ultimatepr.log
chmod 0640 /var/log/ultimatepr.log
install -m 0755 "$SOURCE_DIR/ultimatepr" /opt/ultimatepr/ultimatepr
[ -f /etc/ultimatepr/config.yaml ] && chown ultimatepr:ultimatepr /etc/ultimatepr/config.yaml || chown ultimatepr:ultimatepr /etc/ultimatepr
install -m 0755 "$SOURCE_DIR/ultimatepr-update" /usr/local/sbin/ultimatepr-update

if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
  [ -f "$SOURCE_DIR/ultimatepr.service" ] || { echo "Brak pliku ultimatepr.service." >&2; exit 1; }
  install -d -m 0755 /etc/systemd/system
  install -m 0644 "$SOURCE_DIR/ultimatepr.service" /etc/systemd/system/ultimatepr.service
  systemctl daemon-reload
  systemctl enable --now ultimatepr
  echo "UltimatePR uruchomiono przez systemd."
elif command -v rc-update >/dev/null 2>&1; then
  [ -f "$SOURCE_DIR/ultimatepr.openrc" ] || { echo "Brak pliku ultimatepr.openrc." >&2; exit 1; }
  install -d -m 0755 /etc/init.d
  install -m 0755 "$SOURCE_DIR/ultimatepr.openrc" /etc/init.d/ultimatepr
  rc-update add ultimatepr default
  rc-service ultimatepr restart || rc-service ultimatepr start
  echo "UltimatePR uruchomiono przez OpenRC."
else
  echo "Nie wykryto systemd ani OpenRC. Pliki zainstalowano, ale autostart wymaga ręcznej konfiguracji." >&2
  exit 2
fi

if command -v doas >/dev/null 2>&1; then
  install -d -m 0755 /etc/doas.d
  printf '%s\n' 'permit nopass ultimatepr as root cmd /usr/local/sbin/ultimatepr-update args main' 'permit nopass ultimatepr as root cmd /usr/local/sbin/ultimatepr-update args dev' > /etc/doas.d/ultimatepr.conf
  chmod 0600 /etc/doas.d/ultimatepr.conf
elif command -v sudo >/dev/null 2>&1; then
  install -d -m 0755 /etc/sudoers.d
  printf '%s\n' 'ultimatepr ALL=(root) NOPASSWD: /usr/local/sbin/ultimatepr-update main, /usr/local/sbin/ultimatepr-update dev' > /etc/sudoers.d/ultimatepr
  chmod 0440 /etc/sudoers.d/ultimatepr
  if command -v visudo >/dev/null 2>&1; then
    visudo -cf /etc/sudoers.d/ultimatepr >/dev/null
  fi
else
  echo "Aktualizacja z panelu wymaga pakietu doas albo sudo; sama aplikacja i autostart działają bez niego." >&2
fi

echo "Otwórz http://ADRES_SERWERA:8080 i przejdź konfigurator pierwszego uruchomienia."
