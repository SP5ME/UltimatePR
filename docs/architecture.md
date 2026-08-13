# Architecture

## Dependency direction

```text
Web / Node / Terminal / future BBS
                |
          Session manager
                |
        Router + TX scheduler
                |
      normalized AX.25 frames
                |
 KISS TCP | KISS Serial | AXIP | UDP
```

Application modules never write to a KISS socket. They submit normalized AX.25
frames to a port scheduler. The first scheduler is bounded FIFO; its interface
allows later prioritisation and channel-access policies.

## Packages

- `internal/config`: parsing and strict validation;
- `internal/transport`: transport-neutral port contracts;
- `internal/transport/kiss`: framing and TCP lifecycle only;
- `internal/ax25`: addresses and wire frames, no sockets or Web concepts;
- `internal/session`: link state machines (next increment);
- `internal/monitor`: bounded frame events (next increment);
- `internal/mheard`: bounded station observations (next increment);
- `internal/node`, `terminal`, `web`, `bbs`: applications over sessions.

## Ownership and concurrency

Each physical port owns one RX loop, one bounded TX queue and one writer loop.
Only the writer loop touches the connection for output. Decoded AX.25 frames
are immutable values passed to subscribers. Slow subscribers must not block a
radio port; bounded queues and explicit drop counters are required.

## Security boundaries

KISS and AX.25 data are untrusted. Every parser has a configured maximum size
and returns errors rather than panicking. Web access requires authentication
and can be restricted by an IP/CIDR allowlist. No package
executes a system shell.
