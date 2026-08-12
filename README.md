# Modern Packet Radio BBS

An independent, clean-room implementation of an AX.25 packet-radio node,
router and BBS. The current development milestone includes KISS TCP, AXUDP,
AX.25 modulo-8 sessions, a browser terminal, monitoring, MHEARD and B2F mail
forwarding.

The production program does not import or require the local `Zródał/`
reference laboratory. LinBPQ, URONode and pyBBS are used only to understand
expected user-visible behaviour and interoperability targets.

## Current status

The repository currently contains the first protocol foundation (build
verification is pending installation of the Go toolchain):

- YAML configuration model and validation;
- streaming KISS encoder/decoder with bounds checking;
- KISS TCP client with reconnect and a shared FIFO TX queue;
- AX.25 callsign, address and modulo-8 frame encoder/decoder;
- local Web UI with a real-time dual-mode terminal;
- working bidirectional Telnet BBS connections from the browser;
- unit tests and fuzz targets for protocol parsers;
- incoming AX.25 connections to both the NODE and BBS callsigns;
- B2F/FBB-compatible forwarding with LZHUF compression;
- initial clean-room and architecture documentation.

The AX.25 link layer is still a development implementation (modulo 8, one
outstanding I frame). Test it with Direwolf or another KISS TNC before placing
it on an unattended radio link.

## Build

Requires Go 1.24 or newer.

```sh
go mod download
go test ./...
go vet ./...
go build ./...
```

## Run

```sh
go run ./cmd/server -config configs/example.yaml
```

The example connects to a Direwolf KISS TCP listener on `127.0.0.1:8001` and
serves the local Web UI on `http://127.0.0.1:8080`.

## Browser terminal

Open `http://127.0.0.1:8080`, select a mode and provide connection details.

- **Telnet** is operational and provides a bidirectional connection to a BBS
  reachable by TCP.
- **TNC / Radio** uses the selected KISS port and a basic modulo-8 AX.25
  connected-mode session (SABM/UA, I/RR and DISC/UA). The first implementation
  intentionally uses a one-frame transmit window.

The terminal is a Packet Radio/Telnet client. It never exposes a system shell.

## Safety

Radio and network input is untrusted. Parsers impose explicit size limits.
The project never exposes a system shell. The Web listener defaults to
loopback.

## Documentation

- `docs/INSTRUKCJA-PL.txt` — aktualizowana instrukcja polska
- `docs/USER-MANUAL-EN.txt` — maintained English manual
- `docs/source-analysis.md`
- `docs/feature-reference.md`
- `docs/protocol-sources.md`
- `docs/architecture.md`
# Modern Packet BBS

The server currently provides a local web console, KISS TCP/AX.25 terminal and
a persistent packet BBS service. With the example configuration, connect a
terminal client to `127.0.0.1:8023` and enter your callsign.

Initial BBS commands: `H`, `L`, `LB`, `R <id>`, `S <call>`, `SB <topic>`,
`K <id>` and `B`. End message text with `/EX` on its own line.

An incoming AX.25 connection addressed to the configured node callsign opens
the node command shell. A connection addressed to the configured BBS callsign
opens the mailbox directly and uses the AX.25 source callsign as the user
identity. The same services remain available over their configured Telnet
listeners.

## Windows test start

Double-click `run-test-windows.cmd`, or run from PowerShell:

```powershell
.\run-test-windows.ps1 -OpenBrowser -RunTests
```

Stop the server with `Ctrl+C`.

To start two isolated temporary instances on Windows, run
`run-two-bbs-windows.cmd`. Ctrl+C stops both instances and removes their
temporary configurations, databases and logs.

## Local node test

The example configuration starts a local node on `127.0.0.1:8010`. Connect
from the web terminal in Telnet mode and use: `NODES`, `ROUTES`, `PORTS`,
`SERVICES`, `C BBS` and `BYE`. `C BBS` enters the same persistent BBS and the
BBS command `B` returns to the node.
