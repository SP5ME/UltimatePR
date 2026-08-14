# UltimatePR

**The Ultimate Packet Radio Station**

UltimatePR jest linuksową stacją Packet Radio AX.25 z terminalem WWW, obsługą
KISS TCP i AXUDP, monitorem ramek, MHEARD, historią, beaconem oraz opcjonalnym
pakietem NODE + BBS. NODE i BBS są jednym trybem — nie uruchamia się ich osobno.

## Instalacja wydania na Linuxie

Obsługiwane są Linux AMD64, ARM64 i ARMv7 oraz autostart przez systemd lub
OpenRC (m.in. Alpine Linux). Najprościej uruchomić bootstrap instalatora z
repozytorium:

```sh
curl -fsSLo /tmp/install-ultimatepr.sh https://raw.githubusercontent.com/SP5ME/UltimatePR/main/install-ultimatepr.sh && sh /tmp/install-ultimatepr.sh
```

Jeśli chcesz kanał `dev`, podmień `main` na `dev` w adresie i ostatnim
argumencie skryptu. Gdy masz `sudo`, instalator użyje go automatycznie.

Jeżeli wolisz zainstalować z pobranej paczki, rozpakuj archiwum właściwe dla
architektury i jako root uruchom:

```sh
chmod +x install.sh ultimatepr ultimatepr-update ultimatepr.openrc
./install.sh
```

Instalator tworzy:

- `/opt/ultimatepr/` — program;
- `/etc/ultimatepr/config.yaml` — konfiguracja;
- `/var/lib/ultimatepr/` — dane;
- `/var/lib/ultimatepr/backups/` — kopie przed aktualizacją.

Po instalacji otwórz `http://ADRES_SERWERA:8080`. Przy pierwszym uruchomieniu
pojawi się kreator. Można utworzyć nową, czystą konfigurację albo wczytać kopię
YAML. Nowa konfiguracja nie zawiera znaku ani danych autora projektu.

## Tryby pracy

- `station` — terminal, porty TNC/radio, monitor, MHEARD, historia i beacon;
- `station-node-bbs` — stacja oraz wspólnie uruchamiane NODE i BBS.

## Kopia i aktualizacja

W **Konfiguracja → Aplikacja** można pobrać lub odtworzyć kopię samych ustawień.
Wiadomości BBS i historia nie są częścią tej kopii.

Kanał `main` korzysta z automatycznego pakietu `main-latest` budowanego po
każdym pushu do gałęzi `main`. Kanał `dev` analogicznie korzysta z pakietu
`dev-latest` budowanego z gałęzi `dev`.
Aktualizacja zachowuje konfigurację i dane, zapisuje poprzedni program, wykonuje
restart oraz automatycznie cofa wersję, jeżeli usługa nie wstanie.

UltimatePR działa jako nieuprzywilejowany użytkownik systemowy `ultimatepr`.
Tylko ściśle ograniczony skrypt aktualizacyjny jest wywoływany jako root przez
`doas` albo `sudo`.

## Budowanie i testy

Wymagane jest Go 1.25 lub nowsze.

```sh
go test ./...
go vet ./...
go build -o ultimatepr ./cmd/server
```

Uruchomienie bez istniejącego pliku konfiguracji włącza kreator:

```sh
./ultimatepr -config ./config.yaml
```

Projekt nie wspiera systemu Windows.

## English

UltimatePR is a Linux packet-radio station with a browser terminal, KISS TCP,
AXUDP, frame monitor, MHEARD, history and beacon. It has two operating modes:
station only, or station with NODE and BBS enabled together.

Release packages support Linux AMD64, ARM64 and ARMv7 with systemd or OpenRC.
The simplest install path is:

```sh
curl -fsSLo /tmp/install-ultimatepr.sh https://raw.githubusercontent.com/SP5ME/UltimatePR/main/install-ultimatepr.sh && sh /tmp/install-ultimatepr.sh
```

Replace `main` with `dev` to install the dev channel. If you already unpacked a
release archive, run `./install.sh` as root instead. If `sudo` is available,
the installer will use it automatically. Then open
`http://SERVER_ADDRESS:8080`. The first-run
wizard can create a clean configuration or import a configuration-only YAML
backup. Stable and rolling dev updates preserve configuration and data and
automatically roll back when the service fails its restart check.

Windows is not supported.
