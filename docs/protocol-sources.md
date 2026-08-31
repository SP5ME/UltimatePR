# Protocol sources and open questions

## AX.25

Primary source: ARRL/TAPR AX.25 Link Access Protocol for Amateur Packet Radio,
Version 2.2, July 1998:

https://www.ax25.net/AX25.2.2-Jul%2098-2.pdf

TAPR archive index:

https://tapr.org/ftp-archive/

State diagrams:

https://web.tapr.org/product_docs/tnc95/AX.25.diagram.pdf

The connected-mode implementation currently uses modulo 8 with a one-frame
window. The codec recognizes SABME, SREJ, XID, TEST and FRMR. SREJ triggers a
selective retry of the single outstanding frame; SABME/modulo 128 and a
multi-frame selective-repeat window remain deferred. Session handling validates
command/response and poll/final semantics
for connection setup and release, pauses transmission while the peer reports
RNR, and uses REJ recovery for out-of-sequence I frames.

The default N1 is 256 octets. T1 defaults to 10 seconds for both outgoing and
incoming links, while the terminal T3 poll interval is 5 minutes. N1, T1, T3
and N2 are operator-configurable and the configured N1, T1 and N2 are
advertised and negotiated through XID. After UA, an outgoing link sends an XID
command with P=1 and accepts only an XID response with F=1. N1 and window are
receive-limit notifications; the peer receive N1 limits transmitted frame size.
T1 and N2 use the greater offered value, while duplex, reject mode and modulo
fall back to the mutually supported lower capability. UltimatePR remains
half-duplex, implicit REJ, modulo 8 and window k=1. XID recovery uses the
management data-link parameters TM201=10 seconds and NM201=2 retransmissions
(three transmissions total), independently of the data-link T1 and N2 settings.
A peer that rejects XID with
FRMR or does not respond causes the AX.25 v2.0 compatibility defaults to be
used. Link establishment and release require UA/DM
responses with the Final bit set for locally transmitted SABM/DISC commands
carrying Poll, as required by the AX.25 v2.2 state diagrams.

Traditional AX.25 digipeating can cross configured radio ports. For a frame whose next unrepeated VIA entry is a local alias, UltimatePR selects the most recently heard usable port of the destination. Unknown, same-port, indirect-only, disabled, or disconnected routes fall back to repetition on the input port. Duplicate suppression remains global across ports to prevent reflected frames from looping.

The personal-station client follows the T1 recovery sequence by polling with
RR(P=1) before retransmitting an unacknowledged I frame. Exhausting N2 resets
the local sequence and busy/reject state and reports the link as disconnected.
The T3 probe requires a supervisory response and drops an unresponsive link
after N2 attempts. Remote DM/DISC interrupts an outstanding transmission, and
simultaneous or repeated SABM is handled as connection completion or link reset.

## KISS

Original protocol: Mike Chepponis and Phil Karn, *The KISS TNC: A simple Host
to TNC communications protocol*. The stable framing rules used here are:

- FEND `0xC0` delimits frames;
- the first unescaped byte contains port in the high nibble and command in the
  low nibble;
- FEND in content becomes FESC/TFEND (`DB DC`);
- FESC in content becomes FESC/TFESC (`DB DD`);
- adjacent FEND bytes do not create empty frames.

The transport emits data command `0`. When explicitly configured, it also
emits the portable KISS link-parameter commands TXDELAY, PERSISTENCE, SLOTTIME,
TXTAIL and FULLDUPLEX after each TCP connection is established. Device-specific
SET HARDWARE and RETURN are not emitted.

The optional KISS TCP proxy transparently forwards the TCP byte stream without
parsing or filtering KISS frames and commands. It establishes and maintains its
upstream connection independently of client traffic. Data received from the
upstream TNC is distributed to all clients; data transmitted by one client is
sent to the TNC and the other clients without being echoed to its sender.

## AXIP

IANA assigns IPv4 protocol number 93 to AX.25 encapsulation. Raw IP protocol 93
has no UDP/TCP port and generally needs elevated socket privileges.

https://www.iana.org/assignments/protocol-numbers/protocol-numbers.xhtml

## AX.25 over UDP

AX.25 transported in UDP is not the same mechanism as AXIP protocol 93.
Several implementations use local conventions for peer addressing and optional
headers. No wire format will be implemented until a specific interoperability
target and public format are documented.

## TAPR BBS forwarding

The authoritative source is the TAPR BBS SIG archive:

- https://tapr.org/ftp-archive/?drawer=bbssig*recommendations
- `BBS Specification` / `bbs_spec.doc`
- `BBS Hierarchical Addressing Protocol` / `hierarchical`

The direct forwarding port implements the classical TAPR exchange. Both sides
send a SID; UltimatePR advertises hierarchical addressing, the TAPR null
identification command and BID support as `HI$` (the `$` feature is last as the
specification requires). A master submits `SP`, `SB` or `ST`; the slave answers
`OK` or `NO`. Accepted messages contain a subject, `R:` routing headers, a
blank line, body, and Ctrl-Z end marker. A duplicate bulletin BID is answered
with `NO` and is treated by the sender as already delivered.

After the original master sends `F>`, reverse forwarding begins. The original
slave may send proposals; the original master replies with `OK` or `NO` and
then uses `F>` as the acknowledgement of each accepted or rejected message.
On TCP, where AX.25 does not provide caller identity, the reverse queue is
selected only after a TAPR `I` null identification line matches a configured
peer callsign.

The persistence model follows TAPR terminology: MID identifies a local message
instance, while BID identifies bulletin content across BBS instances. TAPR
x.3.4 hierarchical addresses and bulletin distribution designators are parsed
as different types. B2F/Winlink and private FWD1 are not protocol authorities
for this implementation.
