# Changelog

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
