# Protocol sources and open questions

## AX.25

Primary source: ARRL/TAPR AX.25 Link Access Protocol for Amateur Packet Radio,
Version 2.2, July 1998:

https://tapr.org/pdf/AX25.2.2.pdf

State diagrams:

https://web.tapr.org/product_docs/tnc95/AX.25.diagram.pdf

The connected-mode implementation currently uses modulo 8 with a one-frame
window. The codec recognizes SABME, SREJ, XID, TEST and FRMR. SREJ triggers a
selective retry of the single outstanding frame; SABME/modulo 128, XID
parameter negotiation and a multi-frame selective-repeat window remain
deferred. Session handling validates command/response and poll/final semantics
for connection setup and release, pauses transmission while the peer reports
RNR, and uses REJ recovery for out-of-sequence I frames.

The default N1 is 256 octets. T1 defaults to 10 seconds for both outgoing and
incoming links, while the terminal T3 poll interval is 5 minutes. Reception is
deliberately tolerant of legacy UA/DM responses that omit the Final bit; the
local station still transmits SABM and DISC commands with Poll set as required.

## KISS

Original protocol: Mike Chepponis and Phil Karn, *The KISS TNC: A simple Host
to TNC communications protocol*. The stable framing rules used here are:

- FEND `0xC0` delimits frames;
- the first unescaped byte contains port in the high nibble and command in the
  low nibble;
- FEND in content becomes FESC/TFEND (`DB DC`);
- FESC in content becomes FESC/TFESC (`DB DD`);
- adjacent FEND bytes do not create empty frames.

Until a stable primary copy of the original KISS text is archived in project
documentation, only data command `0` is emitted. Parameter commands remain
future work.

## AXIP

IANA assigns IPv4 protocol number 93 to AX.25 encapsulation. Raw IP protocol 93
has no UDP/TCP port and generally needs elevated socket privileges.

https://www.iana.org/assignments/protocol-numbers/protocol-numbers.xhtml

## AX.25 over UDP

AX.25 transported in UDP is not the same mechanism as AXIP protocol 93.
Several implementations use local conventions for peer addressing and optional
headers. No wire format will be implemented until a specific interoperability
target and public format are documented.

## FBB forwarding

The direct BBS forwarding port implements open B2F with SID capability
negotiation, `FC` proposal blocks, `F>` checksums, `FS` selection, LZHUF
compression and `FF`/`FQ` reverse-forwarding turns. The implementation uses
the MIT-licensed `wl2k-go` protocol and codec package and is adapted to the
UltimatePR store and routing layer. Primary specifications:

- https://www.f6fbb.org/protocole.html
- https://www.winlink.org/B2F

`pyBBS` FWD1 is explicitly not treated as FBB. Classical ASCII/B0/B1 fallback,
secure `;PQ`/`;PR` login and cross-implementation LinBPQ transcripts remain
explicit interoperability gates before external-network release.
