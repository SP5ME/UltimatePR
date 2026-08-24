# Changelog

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
- Outgoing text and transmission status are right-aligned; after a multi-frame message only the final status remains.

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
