# Modern Packet Radio BBS

An independent, clean-room implementation of an AX.25 packet-radio node,
router and, later, BBS. The first milestone focuses on KISS TCP, AX.25 modulo
8 sessions, a browser terminal, monitoring and MHEARD.

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
- initial clean-room and architecture documentation.

The TNC option is visible but deliberately does not transmit yet. It will be
enabled after the connected-mode AX.25 session manager is complete. Do not use
this version on an unattended radio link.

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
