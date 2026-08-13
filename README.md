# UltimatePR

**The Ultimate Packet Radio Station**

[Polski](#polski) | [English](#english)

## Polski

UltimatePR to niezależna implementacja clean-room węzła, routera
i skrzynki BBS dla Packet Radio AX.25. Program jest napisany w Go, ma wbudowany
interfejs WWW i jest dostarczany jako pojedynczy, lekki plik wykonywalny.

### Najważniejsze funkcje

- porty KISS TCP oraz AXUDP;
- sesje AX.25 modulo 8;
- lokalny NODE i trwała skrzynka BBS;
- połączenia BBS przez Telnet i AX.25;
- terminal Packet Radio/Telnet w przeglądarce;
- podgląd ramek, MHEARD i historia połączeń;
- przekazywanie poczty B2F/FBB z kompresją LZHUF;
- polska i angielska wersja komunikatów NODE/BBS;
- konfiguracja w pliku YAML.

Program produkcyjny nie korzysta z lokalnego katalogu laboratoryjnego
`Zródał/`. LinBPQ, URONode i pyBBS służą wyłącznie jako materiały pomocnicze
do analizy zachowania i zgodności protokołów.

### Status projektu

Warstwa łącza AX.25 jest nadal rozwijana. Obecna implementacja obsługuje
modulo 8 oraz jedno oczekujące potwierdzenie ramki I. Przed pozostawieniem
serwera bez nadzoru należy przetestować go z Direwolfem lub innym TNC KISS.

### Wymagania i budowanie

Wymagane jest Go 1.25 lub nowsze.

```sh
go mod download
go test ./...
go vet ./...
go build -o ultimatepr ./cmd/server
```

### Uruchomienie

```sh
go run ./cmd/server -config configs/example.yaml
```

Przykładowa konfiguracja łączy się z KISS TCP Direwolfa pod adresem
`127.0.0.1:8001`. Interfejs WWW jest dostępny na porcie `8080`. Pierwsze
logowanie: użytkownik `admin`, hasło `packet`. Po zalogowaniu zmień hasło w
zakładce **Konfiguracja → Aplikacja**. Dostęp można ograniczyć do pojedynczych
adresów IP lub sieci CIDR, np. `192.168.1.0/24`.

Na Windows można także uruchomić:

```powershell
.\run-test-windows.ps1 -OpenBrowser -RunTests
```

albo dwukrotnie kliknąć `run-test-windows.cmd`. Serwer zatrzymuje się przez
`Ctrl+C`. Skrypt `run-two-bbs-windows.cmd` uruchamia dwie odizolowane,
tymczasowe instancje do testów lokalnych.

### Terminal i usługi

Po otwarciu `http://ADRES_IP_SERWERA:8080` można wybrać tryb połączenia:

- **Telnet** — dwukierunkowe połączenie TCP z serwerem BBS lub NODE;
- **TNC / Radio** — połączenie przez wybrany port KISS i AX.25
  (SABM/UA, I/RR oraz DISC/UA).

Terminal służy wyłącznie do Packet Radio i Telnetu. Nie udostępnia powłoki
systemowej.

Przykładowa konfiguracja uruchamia:

- interfejs WWW: `0.0.0.0:8080`;
- NODE: `127.0.0.1:8010`;
- BBS: `127.0.0.1:8023`;
- port forwarding BBS: `127.0.0.1:7300`.

Podstawowe komendy NODE: `NODES`, `ROUTES`, `PORTS`, `SERVICES`, `C BBS`
i `BYE`.

Podstawowe komendy BBS: `H`, `L`, `LB`, `R <id>`, `S <znak>`,
`SB <temat>`, `K <id>` i `B`. Treść wiadomości kończy się wpisaniem `/EX`
w osobnym wierszu.

### Bezpieczeństwo

Dane radiowe i sieciowe są traktowane jako niezaufane, a parsery stosują
limity rozmiaru. Interfejs WWW wymaga zalogowania i domyślnie nasłuchuje na
`0.0.0.0:8080`.
Przed wystawieniem usług do Internetu należy zastosować zaporę, reverse proxy
z TLS i odpowiednią kontrolę dostępu.

### GitHub Actions i wydania

Workflow CI uruchamia testy, race detector, `go vet` i kompilację przy każdym
pushu oraz pull requeście. Utworzenie tagu w formacie `v*`, na przykład
`v0.4.0`, uruchamia budowanie paczek dla Linuxa oraz tworzy GitHub
Release z sumami SHA-256.

Paczki linuksowe są budowane z `CGO_ENABLED=0`, dlatego plik wykonywalny jest
statyczny i działa również na Alpine Linux bez instalowania glibc.

```sh
git tag v0.4.0
git push origin v0.4.0
```

### Dokumentacja

- `docs/INSTRUKCJA-PL.txt` — pełniejsza instrukcja po polsku;
- `docs/USER-MANUAL-EN.txt` — pełna instrukcja po angielsku;
- `docs/feature-reference.md` — opis funkcji;
- `docs/protocol-sources.md` — źródła specyfikacji;
- `docs/architecture.md` — architektura projektu.

---

## English

UltimatePR is an independent clean-room implementation of an
AX.25 packet-radio node, router and BBS. It is written in Go, includes an
embedded Web interface and is distributed as a single lightweight executable.

### Main features

- KISS TCP and AXUDP ports;
- AX.25 modulo-8 sessions;
- local NODE and persistent BBS mailbox;
- BBS connections over Telnet and AX.25;
- browser-based Packet Radio/Telnet terminal;
- frame monitor, MHEARD and connection history;
- B2F/FBB mail forwarding with LZHUF compression;
- Polish and English NODE/BBS messages;
- YAML configuration.

The production program does not use the local `Zródał/` reference laboratory.
LinBPQ, URONode and pyBBS are used only to study expected user-visible
behaviour and protocol interoperability.

### Project status

The AX.25 link layer is still under development. The current implementation
uses modulo 8 with one outstanding I frame. Test it with Direwolf or another
KISS TNC before placing it on an unattended radio link.

### Requirements and build

Go 1.25 or newer is required.

```sh
go mod download
go test ./...
go vet ./...
go build -o ultimatepr ./cmd/server
```

### Running

```sh
go run ./cmd/server -config configs/example.yaml
```

The example configuration connects to a Direwolf KISS TCP listener at
`127.0.0.1:8001`. The Web interface is served on port `8080`. Initial
credentials are user `admin` and password `packet`. Change the password under
**Configuration → Application** after the first login. Access can be limited
to individual IP addresses or CIDR networks such as `192.168.1.0/24`.

On Windows you can also run:

```powershell
.\run-test-windows.ps1 -OpenBrowser -RunTests
```

or double-click `run-test-windows.cmd`. Stop the server with `Ctrl+C`.
`run-two-bbs-windows.cmd` starts two isolated temporary instances for local
testing.

### Terminal and services

Open `http://SERVER_IP_ADDRESS:8080` and select a connection mode:

- **Telnet** — a bidirectional TCP connection to a BBS or NODE;
- **TNC / Radio** — a connection through the selected KISS port and AX.25
  (SABM/UA, I/RR and DISC/UA).

The terminal is only a Packet Radio/Telnet client. It never exposes a system
shell.

The example configuration starts:

- Web interface: `0.0.0.0:8080`;
- NODE: `127.0.0.1:8010`;
- BBS: `127.0.0.1:8023`;
- BBS forwarding port: `127.0.0.1:7300`.

Basic NODE commands are `NODES`, `ROUTES`, `PORTS`, `SERVICES`, `C BBS` and
`BYE`.

Basic BBS commands are `H`, `L`, `LB`, `R <id>`, `S <call>`, `SB <topic>`,
`K <id>` and `B`. Finish message text by entering `/EX` on a separate line.

### Security

Radio and network input is treated as untrusted and parsers enforce explicit
size limits. The Web listener requires authentication and its address allowlist
can restrict access to trusted hosts or networks. Before
exposing services to the Internet, use a firewall, a TLS reverse proxy and
appropriate access controls.

### GitHub Actions and releases

The CI workflow runs tests, the race detector, `go vet` and a build on every
push and pull request. Creating a `v*` tag, for example `v0.4.0`, builds
Linux packages and publishes a GitHub Release with SHA-256
checksums.

Linux packages are built with `CGO_ENABLED=0`, so the executable is static and
runs on Alpine Linux without installing glibc.

```sh
git tag v0.4.0
git push origin v0.4.0
```

### Documentation

- `docs/INSTRUKCJA-PL.txt` — complete Polish manual;
- `docs/USER-MANUAL-EN.txt` — maintained English manual;
- `docs/feature-reference.md` — feature reference;
- `docs/protocol-sources.md` — protocol specification sources;
- `docs/architecture.md` — project architecture.
