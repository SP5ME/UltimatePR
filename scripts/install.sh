#!/bin/sh
set -eu

[ "$(id -u)" -eq 0 ] || { echo "Uruchom instalator jako root." >&2; exit 1; }
SOURCE_DIR=$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)
[ -x "$SOURCE_DIR/ultimatepr" ] || { echo "Brak pliku ultimatepr obok instalatora." >&2; exit 1; }

if ! id ultimatepr >/dev/null 2>&1; then
  if command -v adduser >/dev/null 2>&1; then adduser -S -D -H -s /sbin/nologin ultimatepr 2>/dev/null || adduser --system --no-create-home --shell /usr/sbin/nologin ultimatepr
  else useradd --system --no-create-home --shell /usr/sbin/nologin ultimatepr; fi
fi
install -d -m 0755 /opt/ultimatepr /etc/ultimatepr
install -d -m 0750 -o ultimatepr -g ultimatepr /var/lib/ultimatepr /var/lib/ultimatepr/backups
install -m 0755 "$SOURCE_DIR/ultimatepr" /opt/ultimatepr/ultimatepr
[ -f /etc/ultimatepr/config.yaml ] && chown ultimatepr:ultimatepr /etc/ultimatepr/config.yaml || chown ultimatepr:ultimatepr /etc/ultimatepr
[ -f "$SOURCE_DIR/ultimatepr-update" ] && install -m 0755 "$SOURCE_DIR/ultimatepr-update" /usr/local/sbin/ultimatepr-update

if command -v systemctl >/dev/null 2>&1 && [ -d /run/systemd/system ]; then
  install -m 0644 "$SOURCE_DIR/ultimatepr.service" /etc/systemd/system/ultimatepr.service
  systemctl daemon-reload
  systemctl enable --now ultimatepr
  echo "UltimatePR uruchomiono przez systemd."
elif command -v rc-update >/dev/null 2>&1; then
  install -m 0755 "$SOURCE_DIR/ultimatepr.openrc" /etc/init.d/ultimatepr
  rc-update add ultimatepr default
  rc-service ultimatepr restart || rc-service ultimatepr start
  echo "UltimatePR uruchomiono przez OpenRC."
else
  echo "Nie wykryto systemd ani OpenRC. Pliki zainstalowano, ale autostart wymaga ręcznej konfiguracji." >&2
  exit 2
fi

if command -v doas >/dev/null 2>&1; then
  printf '%s\n' 'permit nopass ultimatepr as root cmd /usr/local/sbin/ultimatepr-update args main' 'permit nopass ultimatepr as root cmd /usr/local/sbin/ultimatepr-update args dev' > /etc/doas.d/ultimatepr.conf
  chmod 0600 /etc/doas.d/ultimatepr.conf
elif command -v sudo >/dev/null 2>&1; then
  printf '%s\n' 'ultimatepr ALL=(root) NOPASSWD: /usr/local/sbin/ultimatepr-update main, /usr/local/sbin/ultimatepr-update dev' > /etc/sudoers.d/ultimatepr
  chmod 0440 /etc/sudoers.d/ultimatepr
else
  echo "Aktualizacja z panelu wymaga pakietu doas albo sudo; sama aplikacja i autostart działają bez niego." >&2
fi

echo "Otwórz http://ADRES_SERWERA:8080 i przejdź konfigurator pierwszego uruchomienia."
