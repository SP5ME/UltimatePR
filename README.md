# UltimatePR

**The Ultimate Packet Radio Station**

UltimatePR jest linuksową stacją Packet Radio AX.25 z terminalem WWW, obsługą
KISS TCP i AXUDP, monitorem ramek, MHEARD, historią, beaconem oraz opcjonalnym
pakietem NODE + BBS. NODE i BBS są jednym trybem — nie uruchamia się ich osobno.

## Założenia i najważniejsze funkcje

UltimatePR ma być jedną, samodzielną usługą do obsługi stacji Packet Radio:
konfiguracja, praca operatora i diagnostyka są dostępne w jednym panelu WWW.
Aplikacja nie wymaga linuksowego stosu AX.25 — ramki i sesje są obsługiwane
wewnątrz programu, a TNC jest dostępne przez KISS TCP lub AXUDP.

Najważniejsze możliwości:

- terminal WWW z wieloma sesjami, historią i tekstem UTF-8;
- dwuwierszowe pole wpisywania, zawijanie długiego tekstu i pionowe przewijanie;
- `Enter` wysyła tekst, a `Shift+Enter` dodaje ręczny podział wiersza;
- monitor ramek RX/TX, MHEARD, historia i automatyczny beacon;
- MHEARD z kolorem aktywności oraz wyrównanym do prawej czasem w minutach od
  ostatniej odebranej ramki;
- KISS TCP z reconnectem, portami KISS 0–15 i parametrami TXDELAY,
  PERSISTENCE, SLOTTIME, TXTAIL oraz FULLDUP;
- AXUDP z opcjonalnym FCS i filtrowaniem peerów;
- tryb samej stacji albo wspólny NODE + BBS;
- kreator pierwszego uruchomienia, interfejs PL/EN i kopie ustawień YAML;
- kanały aktualizacji `main` i `dev` z WWW oraz linii poleceń.

Konfiguracja i dane są przechowywane poza programem, dlatego aktualizacja ich
nie zastępuje. Bezpośrednie TNC szeregowe RS-232 nie jest obecnie obsługiwane;
można użyć mostu udostępniającego je jako KISS TCP.

## Instalacja wydania na Linuxie

Gotowe paczki obsługują Linux AMD64, ARM64 i ARMv7. Instalator konfiguruje
systemd na Debianie, Ubuntu, Raspberry Pi OS, Fedorze, Arch Linux i innych
dystrybucjach z systemd albo OpenRC na Alpine Linux i systemach pochodnych.
Wymaga `sh`, `tar`, `curl` albo `wget` oraz `sha256sum` albo
`shasum`. Aktualizacja z WWW wymaga również `sudo` lub `doas`.

Domyślnym kanałem instalacji jest `main`:

```sh
(command -v curl >/dev/null 2>&1 && curl -fsSLo /tmp/install-ultimatepr.sh https://raw.githubusercontent.com/SP5ME/UltimatePR/main/install-ultimatepr.sh || wget -qO /tmp/install-ultimatepr.sh https://raw.githubusercontent.com/SP5ME/UltimatePR/main/install-ultimatepr.sh) && sh /tmp/install-ultimatepr.sh
```

Kanał `dev` zainstalujesz poleceniem:

```sh
(command -v curl >/dev/null 2>&1 && curl -fsSLo /tmp/install-ultimatepr.sh https://raw.githubusercontent.com/SP5ME/UltimatePR/dev/install-ultimatepr.sh || wget -qO /tmp/install-ultimatepr.sh https://raw.githubusercontent.com/SP5ME/UltimatePR/dev/install-ultimatepr.sh) && sh /tmp/install-ultimatepr.sh dev
```

Końcowy argument `dev` jest wymagany — bez niego instalator wybiera domyślny
kanał `main`. Gdy masz `sudo`, instalator użyje go automatycznie.

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

## Terminal i TNC przez sieć

UltimatePR komunikuje się z TNC przez KISS TCP lub AXUDP; bezpośrednie TNC
szeregowe RS-232 nie jest obecnie przewidziane. Opcjonalne TNC Proxy udostępnia
jeden port KISS TCP aplikacji i zewnętrznym klientom, rozdzielając pełne ramki
KISS i chroniąc wspólne TNC przed komendami zmieniającymi jego stan.

Proxy utrzymuje jedno połączenie z TNC i udostępnia osobny port klientom.
Ramka odebrana z TNC trafia do wszystkich klientów. Ramka wysłana przez jednego
klienta trafia do TNC i pozostałych klientów, ale nie wraca do nadawcy. Dzięki
temu monitor widzi transmisje innych aplikacji — zgodnie z efektem proxy SQ5T
opisanym przez SQ9MDD. Proxy przekazuje kompletne ramki KISS, automatycznie
odnawia połączenie oraz blokuje `SET HARDWARE` i `RETURN`. Dostęp klientów
kontroluje `web.allowed_addresses`.

```yaml
ports:
  - id: radio-2m
    type: kiss-tcp
    host: 127.0.0.1
    port: 8001
    tncproxy_enabled: true
    tncproxy_port: 8101
```

UltimatePR i programy zewnętrzne łączą się z portem `8101`, a proxy z TNC na
`8001`.

W **Konfiguracja → Terminal** można ustawić zakończenie wiersza, T1, T3, N2
oraz N1/PACLEN. Każde pole ma krótki opis i podaną wartość domyślną, a przycisk
przywracania ustawia profil: CR, T1=10 s, T3=300 s, N2=10 i N1=256 B.
Tekst terminala pozostaje w UTF-8.

## Kopia i aktualizacja

Szczegóły zgodności KISS, obsługiwanych komend TNC, portów KISS 0–15 oraz
kodowań terminala znajdują się w [docs/kiss-terminal.md](docs/kiss-terminal.md).

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

### Aktualizacja z linii poleceń

```sh
sudo /usr/local/sbin/ultimatepr-update main
sudo /usr/local/sbin/ultimatepr-update dev
```

Z `doas`:

```sh
doas /usr/local/sbin/ultimatepr-update main
doas /usr/local/sbin/ultimatepr-update dev
```

Diagnostyka systemd:

```sh
systemctl status ultimatepr
journalctl -u ultimatepr -n 100 --no-pager
sudo systemctl restart ultimatepr
```

Diagnostyka OpenRC:

```sh
rc-service ultimatepr status
tail -n 100 /var/log/ultimatepr.log
doas rc-service ultimatepr restart
```

Kanał zapisany w panelu dotyczy aktualizacji z WWW. Argument komendy wybiera
kanał dla konkretnego uruchomienia aktualizatora.

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

## Goals and highlights

UltimatePR is one self-contained service for station configuration, operation
and diagnostics. It does not require the Linux AX.25 stack; framing and session
handling run inside the application.

Highlights include:

- browser terminal with multiple sessions, history and UTF-8 text;
- a two-line composer with wrapping and vertical scrolling;
- `Enter` sends, while `Shift+Enter` inserts a manual line break;
- RX/TX monitor, MHEARD, history and automatic beacon;
- MHEARD activity colour and right-aligned minutes since the last frame;
- reconnecting KISS TCP, KISS ports 0–15 and link-parameter commands;
- AXUDP with optional FCS and peer filtering;
- station-only or combined NODE + BBS mode;
- first-run wizard, Polish/English UI and YAML settings backups;
- `main` and `dev` updates from the web interface or command line.

Configuration and user data are stored outside the executable, so updates do
not replace them.

TNC communication uses network transports: KISS TCP or AXUDP. Direct RS-232
TNC support is not currently planned. The optional TNC Proxy safely shares one
KISS TCP endpoint between UltimatePR and external clients. It distributes TNC
frames to every client and sends a client's frame to the TNC and every other
client without echoing it to the sender. Complete KISS frames are preserved
across TCP reads, reconnect is automatic, and `SET HARDWARE` plus `RETURN`
are blocked. Access is controlled by `web.allowed_addresses`.

```yaml
ports:
  - id: radio-2m
    type: kiss-tcp
    host: 127.0.0.1
    port: 8001
    tncproxy_enabled: true
    tncproxy_port: 8101
```

The dedicated
**Configuration → Terminal** tab documents and configures line endings, T1,
T3, N2, and N1/PACLEN. Its restore button applies CR, T1=10 s, T3=300 s,
N2=10, and N1=256 bytes. Terminal text remains UTF-8.

Release packages support Linux AMD64, ARM64 and ARMv7 with systemd or OpenRC.
The default install channel is `main`:

```sh
(command -v curl >/dev/null 2>&1 && curl -fsSLo /tmp/install-ultimatepr.sh https://raw.githubusercontent.com/SP5ME/UltimatePR/main/install-ultimatepr.sh || wget -qO /tmp/install-ultimatepr.sh https://raw.githubusercontent.com/SP5ME/UltimatePR/main/install-ultimatepr.sh) && sh /tmp/install-ultimatepr.sh
```

Install the `dev` channel with:

```sh
(command -v curl >/dev/null 2>&1 && curl -fsSLo /tmp/install-ultimatepr.sh https://raw.githubusercontent.com/SP5ME/UltimatePR/dev/install-ultimatepr.sh || wget -qO /tmp/install-ultimatepr.sh https://raw.githubusercontent.com/SP5ME/UltimatePR/dev/install-ultimatepr.sh) && sh /tmp/install-ultimatepr.sh dev
```

The final `dev` argument is required; without it, the installer selects the
default `main` channel. If you already unpacked a release archive, run
`./install.sh` as root instead. If `sudo` is available, the installer will use
it automatically. Then open `http://SERVER_ADDRESS:8080`. The first-run
wizard can create a clean configuration or import a configuration-only YAML
backup. Stable and rolling dev updates preserve configuration and data and
automatically roll back when the service fails its restart check.

Command-line updates:

```sh
sudo /usr/local/sbin/ultimatepr-update main
sudo /usr/local/sbin/ultimatepr-update dev
```

With `doas`:

```sh
doas /usr/local/sbin/ultimatepr-update main
doas /usr/local/sbin/ultimatepr-update dev
```

Useful service diagnostics:

```sh
systemctl status ultimatepr
journalctl -u ultimatepr -n 100 --no-pager
rc-service ultimatepr status
tail -n 100 /var/log/ultimatepr.log
```

Prebuilt AMD64, ARM64 and ARMv7 packages support Debian, Ubuntu, Raspberry Pi
OS, Fedora, Arch Linux and other systemd distributions, plus Alpine Linux and
other OpenRC systems.

Windows is not supported.
