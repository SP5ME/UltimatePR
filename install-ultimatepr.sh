#!/bin/sh
set -eu

REPO=${ULTIMATEPR_REPO:-SP5ME/UltimatePR}
CHANNEL=${1:-main}

case "$CHANNEL" in
  main|dev) ;;
  *)
    echo "Użycie: sh install-ultimatepr.sh [main|dev]" >&2
    exit 1
    ;;
esac

if [ "$(id -u)" -ne 0 ]; then
  if command -v sudo >/dev/null 2>&1; then
    exec sudo sh "$0" "$@"
  fi
  echo "Uruchom instalator jako root, np. sh install-ultimatepr.sh $CHANNEL." >&2
  exit 1
fi

case "$(uname -m)" in
  x86_64|amd64) ARCH=amd64 ;;
  aarch64|arm64) ARCH=arm64 ;;
  armv7l|armv7*) ARCH=armv7 ;;
  *)
    echo "Obsługiwane są tylko amd64, arm64 i armv7." >&2
    exit 1
    ;;
esac

if command -v curl >/dev/null 2>&1; then
  FETCH=curl
elif command -v wget >/dev/null 2>&1; then
  FETCH=wget
else
  echo "Wymagany jest curl albo wget." >&2
  exit 1
fi

if command -v sha256sum >/dev/null 2>&1; then
  SHA256=sha256sum
elif command -v shasum >/dev/null 2>&1; then
  SHA256='shasum -a 256'
else
  echo "Wymagany jest sha256sum albo shasum." >&2
  exit 1
fi

download() {
  url=$1
  out=$2
  attempt=1
  while [ "$attempt" -le 5 ]; do
    if [ "$FETCH" = curl ]; then
      if curl -fsSL -o "$out" "$url"; then
        return 0
      fi
    else
      if wget -qO "$out" "$url"; then
        return 0
      fi
    fi
    sleep "$attempt"
    attempt=$((attempt + 1))
  done
  return 1
}

checksum_of() {
  file=$1
  if [ "$SHA256" = sha256sum ]; then
    sha256sum "$file" | awk '{print $1}'
  else
    shasum -a 256 "$file" | awk '{print $1}'
  fi
}

PKG="ultimatepr-${CHANNEL}-linux-${ARCH}"
BASE_URL="https://github.com/${REPO}/releases/download/${CHANNEL}-latest"
TMPDIR=$(mktemp -d)
cleanup() {
  rm -rf "$TMPDIR"
}
trap cleanup EXIT INT TERM

ARCHIVE="$TMPDIR/$PKG.tar.gz"
CHECKSUMS="$TMPDIR/SHA256SUMS.txt"
EXTRACT="$TMPDIR/extract"

echo "Pobieranie UltimatePR ${CHANNEL} dla ${ARCH}..."
download "$BASE_URL/$PKG.tar.gz" "$ARCHIVE" || {
  echo "Nie udało się pobrać archiwum $PKG.tar.gz." >&2
  exit 1
}
download "$BASE_URL/SHA256SUMS.txt" "$CHECKSUMS" || {
  echo "Nie udało się pobrać SHA256SUMS.txt." >&2
  exit 1
}

EXPECTED=$(awk -v file="$PKG.tar.gz" '$2 == file { print $1; exit }' "$CHECKSUMS")
[ -n "$EXPECTED" ] || {
  echo "Nie znaleziono sumy kontrolnej dla $PKG.tar.gz." >&2
  exit 1
}
ACTUAL=$(checksum_of "$ARCHIVE")
[ "$EXPECTED" = "$ACTUAL" ] || {
  echo "Checksum nie zgadza się dla $PKG.tar.gz." >&2
  exit 1
}

mkdir -p "$EXTRACT"
tar -xzf "$ARCHIVE" -C "$EXTRACT"
SRC="$EXTRACT/$PKG"
[ -d "$SRC" ] || {
  echo "Brak katalogu $PKG w archiwum." >&2
  exit 1
}
[ -f "$SRC/install.sh" ] || {
  echo "W archiwum brakuje install.sh." >&2
  exit 1
}

chmod +x "$SRC/install.sh" "$SRC/ultimatepr" "$SRC/ultimatepr-update" "$SRC/ultimatepr.openrc" 2>/dev/null || true
cd "$SRC"
sh ./install.sh
