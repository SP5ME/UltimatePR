# Game Hall

Game Hall is an optional UltimatePR service for small multiplayer games over
AX.25 terminal links. The first release supplies the shared infrastructure and
one reference game, Tic-Tac-Toe. It deliberately has no rankings, tournaments,
bots, persistence, word packs or graphical game client.

## Architecture

`internal/gamehall` has four distinct responsibilities:

1. `Hall` tracks connected players, invitations and shared sessions.
2. `GameSession` contains common metadata and lifecycle state (`invited`,
   `active`, `finished`, `cancelled`, `disconnected`).
3. A `Game` implementation owns authoritative rules and private server data.
4. `View(player)` and the terminal renderer expose only player-visible state.

The distinction between the full `Game` state and its per-player `View` is a
hard boundary. A future Hangman word, card hand or Battleships layout stays in
server state and must not be copied automatically to a client view. Shared
content/word/phrase packs should later be registered as Hall resources rather
than embedded in one game.

Games use generic invitation, action, state, end, rematch and leave operations.
To add a game, implement `Game`, provide a factory, register its `GameType`, and
add a compact renderer. The lobby and session lifecycle do not need redesign.

## Configuration and access

The service uses the existing `experimental.services` gate and AX.25 inbound
service multiplexer. Example:

```yaml
experimental: {services: true}
game_hall:
  enabled: true
  callsign: SP5ME
  ssid: 14
  language: pl
  invite_timeout_seconds: 120
```

SSID 14 is only a default and accepts the normal AX.25 range 0..15. Connect
directly to the resulting address (for example `SP5ME-14`). When NODE is
enabled, Game Hall is also registered automatically as `GAME`, available as
`GAME` or `C GAME`.

## Terminal protocol

All essential interaction is 7-bit ASCII with CRLF line endings. Lobby commands
are `GAMES`, `PLAYERS`, `INVITES`, `PLAY <call>`, `ACCEPT <id>`,
`DECLINE <id>`, `HELP`, and `QUIT`. Invitation expiry defaults to two minutes.

During Tic-Tac-Toe, enter a coordinate such as `B2`. `BOARD` redraws the small
3x3 board, `HELP` shows the rules, and `QUIT` returns to the lobby without
disconnecting. After a result, `REMATCH` requests or accepts another game; both
players must agree. The server validates turn, coordinate, occupancy, win and
draw. A disconnect interrupts the session, notifies the peer and releases both
players.

The complete board is sent after each move because it is smaller and safer than
a delta protocol here. Future larger games may send actions plus occasional
public-view synchronization.

## First-version limits

Presence and sessions are in memory and disappear on restart. Only one pending
invitation or game per player is allowed. There is no spectator mode, Web game
view, reconnection/resume, persistence, administration UI for content packs, or
additional game implementation.
