# Clean-room source analysis

The projects in `Zródał/` are a local, read-only laboratory. They are not
dependencies and are not distributed with this project.

## LinBPQ

### License

No project-wide license was found in the repository root. Individual bundled
components contain their own notices, which do not license LinBPQ as a whole.
The code is therefore treated as all-rights-reserved reference material.

### Relevant behaviour

- one switch coordinates many radio and IP ports;
- KISS is a byte-stream framing layer, while AX.25 logic remains in the link
  layer;
- applications share the same ports through central buffering and dispatch;
- monitoring and MHEARD are derived from frames crossing the switch;
- node, BBS and terminal sessions are separate applications over a common
  session/port core.

### Use in this project

Only user-visible behaviour, interoperability experiments and failure cases
may inform requirements. Source, structures and implementation algorithms are
not copied.

## URONode

### License

GPL version 2. It is not linked with or copied into this MPL-2.0 project.

### Relevant behaviour

URONode provides a small Unix-oriented session shell with an explicit command
registry, user context and gateways to AX.25, NET/ROM, ROSE and TCP. It relies
on the Linux kernel AX.25 stack rather than providing a portable link layer.

### Use in this project

The separation of session, command and gateway responsibilities is a useful
architectural lesson. Behaviour is independently implemented.

## pyBBS

### License

MIT, Copyright (c) 2026 Rysiek Labus (SQ9MDD).

### Relevant behaviour

The prototype demonstrates users, private messages, bulletins, a durable
outbox, BID duplicate detection, neighbour health, topology and multi-hop mail
routing in SQLite. Its `FWD1` protocol is a private TCP protocol, not FBB and
not AXIP.

### Use in this project

Its BBS concepts may inform a later data model. The first milestone does not
copy or port its code and does not implement BBS storage.

