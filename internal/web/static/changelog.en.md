# Changelog

## 2026-08-26 - AX.25 terminal and safe TNC Proxy

- Completed post-connect XID negotiation: N1 respects the peer receive limit, T1 and N2 select the greater value, and unsupported features are reduced to the modulo-8, implicit-REJ, k=1 profile.
- A missing XID response or FRMR selects the AX.25 v2.0 compatibility parameters; the implicit-REJ XID function mask was also corrected.
- Added a dedicated `Terminal` configuration tab with plain-language descriptions, visible defaults, and a button that restores the complete default profile.
- Configurable settings now include `CR` / `CRLF` / `LF` line endings, T1 response time, T3 idle-link checking, N2 retry count, and N1/PACLEN maximum data size.
- The default profile remains aligned with the implemented AX.25 modulo-8 mode: CR, T1=10 s, T3=300 s, N2=10, and N1=256 bytes.
- XID responses advertise the configured N1, T1, and N2 values; settings are validated before the configuration is saved.
- The terminal preserves UTF-8 text without automatically transliterating Polish characters to ASCII.
- Pressing Enter can send an empty line; transmissions use one ordered queue and can be safely cancelled while disconnecting.
- Outgoing messages are added to history only after successful transmission.
- TNC Proxy separates the stream into complete KISS frames, does not echo client data to other clients, and distributes frames received from the TNC to every client.
- The proxy maintains and retries its TCP connection to the TNC and blocks the KISS `SET HARDWARE` and `RETURN` commands that could alter shared-device state.

## 2026-08-24 - AX.25 recovery timer correction

- Unified the default `T1` to 10 seconds for outgoing and incoming links and for the parameters advertised through `XID`. The 3-second value caused premature retransmissions with full `N1=256` frames and delayed remote stations.

## 2026-08-24 - AX.25 link release fix

- Receiving a remote `DISC`, `DM`, or `SABM` now immediately ends the pending link-recovery procedure and prevents frames from being sent after the session has closed.

## 2026-08-24 - diagnostic monitor export

- Added a monitor export format selector: the existing human-readable TXT format and RAW JSONL for AX.25 frame diagnostics.
- RAW exports preserve complete monitor entries in chronological order, including the encoded frame bytes.

## 2026-08-24 - AX.25 compliance and stability

- Unified AX.25 protocol handling for outgoing terminal links and incoming service links; the protocol layer is independent of terminal, BBS, and NODE application functions.
- Fixed routing for frames addressed to an active outgoing link: LinBPQ banners and other `I` frames now reach the correct terminal session and receive `RR`, instead of being claimed by inbound-service handling and rejected with `DM`.
- Delayed valid `UA(F=1)` responses to retried `SABM(P=1)` commands no longer tear down a newly established link; unrelated unexpected UA frames outside the establishment period still invoke protocol error handling.
- Fixed the direction of information frames sent by incoming sessions: `I` frames are now correctly marked as commands instead of responses.
- Added the `Timer Recovery` state and complete `V(S)`, `V(A)`, and `V(R)` tracking for the active modulo-8 profile.
- Completed personal-station T1 recovery: an `RR(P=1)` poll is sent before retransmitting an unacknowledged I frame, and N2 exhaustion disconnects the link and clears sequence, RNR, and REJ state.
- Incoming sessions use the same enquiry-before-retransmission procedure and close local links with `DISC(P=1)`, retrying until `UA(F=1)` or `DM(F=1)` is received.
- T3 keepalive now requires a response from the remote station and detects a lost connection instead of merely transmitting a control frame.
- Remote `DM` and `DISC` immediately interrupt an outstanding transmission, while simultaneous or repeated `SABM` correctly completes or resets the link.
- Added a confirmed Clear monitor action that removes all currently buffered frames without stopping subsequent monitoring.
- Link establishment and release now strictly require `UA/DM` responses carrying `F=1` for transmitted `SABM/DISC` commands carrying `P=1`.
- Fixed monitor frame names after extending the codec — `SABM`, `UA`, `RR`, `I`, and the remaining types are no longer shifted or incorrectly displayed as `?`.
- Applied the AX.25 v2.2 defaults: T1=10 seconds, N2=10 retries, N1=256 octets, and T3=5 minutes.
- Fixed `RNR` handling: information-frame transmission now pauses while the remote station reports receiver busy, and readiness is checked using `RR` frames with the `P` bit.
- Out-of-sequence information frames now start controlled `REJ` recovery instead of receiving an ordinary `RR` acknowledgement.
- Added `SREJ` handling for the current one-frame transmit window.
- Session setup, information transfer, and release validate normative `P/F` and `C/R` semantics; invalid combinations cannot establish a session.
- Incoming sessions answer supervisory polls carrying the `P` bit and send `DM` when a local service is in the disconnected state.
- The codec recognizes additional AX.25 v2.2 frame types: `SABME`, `SREJ`, `FRMR`, `XID`, and `TEST`.
- Added encoding and decoding of the two-octet control field used by optional modulo 128. The session engine continues to negotiate the stable modulo-8 profile and rejects unsupported `SABME` setup with `DM`.
- Added `XID` handling that advertises the actual profile: half duplex, implicit REJ, modulo 8, N1=256, window k=1, T1=10 seconds, and N2=10.
- Added `TEST` responses that preserve the information field and the required responses to `UI(P=1)`.
- In accordance with AX.25 v2.2, link errors are handled through link reset; the application does not generate the `FRMR` response removed by this version.
- The encoder rejects information fields on control frames that do not permit them.
- Added regression coverage for `RNR`, `REJ`, `SREJ`, `P/F`, `C/R`, XID, TEST, modulo 128, supervisory polling, `DM` responses, controlled link release, outgoing-session routing, and delayed UA responses.

## 2026-08-24 - hostnames, independent sessions, and clearer history

- Added deletion of the currently viewed conversation and all conversation history from the new Database tab; every deletion requires confirmation.
- “Clear terminal” clears only the current terminal window after confirmation and leaves saved history untouched.

- `web.allowed_addresses` now accepts hostnames in addition to IP addresses and CIDR networks. The same rules protect the web panel and TNC Proxy clients, and names are resolved through DNS whenever a connection is checked.
- Fixed hostname access for clients using scoped IPv6 link-local addresses such as `fe80::…%eth0`; the `::` rule now covers them correctly in both the web panel and TNC Proxy.
- Every terminal tab has its own connection state and a dedicated `×` button that permanently closes the session.
- The `Disconnect session` button only ends the current connection, preserving the tab, terminal contents, and manual reconnect option.
- Automatic reconnection and its main-bar switch were removed. The new-connection fields now use all available space.
- History and beacon previews are separated from active sessions, so they neither display nor inherit another connection's state.
- The viewed history title is prominent and centered in the terminal header.
- History messages show one date and time for the complete message instead of separate transmission packets.
- The beginning and end of every connection are stored permanently and displayed as graphical separators: green `Connected` and red `Disconnected`, both with date and time.
- Manually disconnecting or closing an active tab now sends the configured goodbye first, waits for its transmission to finish, and only then sends the AX.25 `DISC` frame.
- Readability is now consistent across the interface: the smallest descriptions, metadata, statuses, and hints are larger, heavier, and higher-contrast in both light and dark themes.
- Before saving a changed web-panel address, the application verifies that it can open it. Missing permissions or an occupied port no longer overwrite a working configuration or lock out the panel after restart.

## 2026-08-21 - sharing TNC access with external applications

- Added an optional built-in KISS TCP proxy for each TNC port.
- With the proxy disabled, UltimatePR connects directly to the TNC, for example on port `8001`.
- With the proxy enabled, UltimatePR and external KISS applications can share the client port, by default `127.0.0.1:8101`.
- Added `tncproxy_enabled` and `tncproxy_port` settings to the TNC tab and YAML configuration.
- The `web.allowed_addresses` list also limits TNC Proxy clients.
- The proxy forwards frames between the TNC and all connected clients.

## 2026-08-21 - safer remote commands

- Remote commands now require a `/` prefix and a separate line: `/I` and `/MH`.
- Added help through `/H` and `/?`.
- The default welcome message informs the correspondent about available commands; custom welcomes are not overwritten.
- Long messages are split at the last space or line ending before the `paclen` limit, without cutting ordinary words.
- Outgoing text and transmission status are right-aligned; every packet of a multi-frame message remains visible on its own line with its status.

## 2026-08-21 - session stability and monitor export

- Old or unrelated `RR` acknowledgements no longer cause immediate repeated retransmissions of the same message.
- `REJ` still starts a controlled retry of the correct frame according to the sequence number.
- The frame monitor can be exported to a readable TXT file in chronological order.

## 2026-08-21 - terminal macros

- The `{CALL}`, `{NAME}`, `{LOC}`, `{QTH}` and `{REMOTE}` macros are also expanded in messages entered during a conversation, immediately before sending.
- Macro data is read from the current configuration in the `Operator station` section.
- The goodbye message shown after a remote disconnect now displays substituted values instead of raw macro names.

## 2026-08-21 - incoming connections through DIGI

- Incoming sessions remember the DIGI route received in the `SABM` frame.
- Return `UA`, `RR`, `I` and `DISC` frames are sent through the reversed route, allowing the correspondent to receive and acknowledge them.
- Fixed repeated welcome messages and later `DISC` / `SABM` loops during connections through a digipeater.
- Polish characters entered in the conversation field are converted to readable ASCII equivalents before transmission, for example `ąśćę` to `asce`.

## 2026-08-20

- Replaced text labels with top-bar icons for theme, sound, information and configuration.
- Added the `Info` view with a simple manual and the application's main principles.
- Added `Changelog` as a sub-tab in `Info`.
- Added `MH` and `I` / `INFO` commands with automatic terminal responses.
- Added editable terminal messages: welcome, goodbye and info.

## 2026-08-20 - visual cleanup

- Made cards in the `Info` view lighter and easier to read.
- Made terminal message editing fields light and easier to read.
- Improved changelog text contrast.

## 2026-08-20 - info cleanup

- Removed the application status panel from `Info`.
- Removed `uptime` from the top bar and `Info`.
- Added a GitHub repository link at the bottom of the `Info` view.
