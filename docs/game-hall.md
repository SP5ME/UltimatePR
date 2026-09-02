# Game Hall

Game Hall is an optional UltimatePR service for terminal games over AX.25.
It is designed first as an ASCII service with CRLF line endings and only then
as something a GUI can render.

## Model

The service is split into small, explicit pieces:

1. `GameDefinition` registers one game in the lobby.
2. `Hall` keeps connected players, invitations, rooms and active sessions.
3. `GameSession` stores authoritative server state for one running game.
4. `GameRoom` stores the staging area for ROOM-style games.
5. `Invitation` stores pending INVITE-style challenges.
6. `Game` owns the rules and the hidden server state.
7. `PlayerView` is the client-facing view for one player.
8. `GameAction` is the generic shape for future transport actions.

The boundary between server state and player view is intentional. A game may
keep secret data only on the server and expose a reduced `PlayerView` instead.
This is the place for future Hangman words, card hands or Battleships ship
layouts without leaking them over radio.

The current public/server split is:

- `PUBLIC`
- `SERVER_SECRET`

`PUBLIC` data may be shared with both players. `SERVER_SECRET` stays inside the
game implementation.

## Join modes

Every game declares a join mode:

- `SOLO` - one player starts immediately.
- `INVITE` - one player invites a specific opponent.
- `ROOM` - a host creates a room and other players join it.

The reference game, Tic-Tac-Toe, uses `INVITE`.

## Lobby

After connecting, the operator lands in the main lobby. The lobby lists all
registered games from the `Hall` registry. The list is generated from the
registered definitions, not hardcoded in the terminal loop.

Typical commands:

- `GAMES` or a numeric entry - choose a game.
- `PLAYERS` - show connected players and their status.
- `INVITES` - show pending invitations.
- `HELP` - show the lobby commands.
- `QUIT` - leave the service.

The terminal prompt stays context-aware:

- `GAME>` in the main lobby.
- `TICTACTOE>` in the selected game lobby.
- `ROOM#01>` in a room.
- `INVITES>` in the invitation list.

## Invite flow

`INVITE` games let one player challenge another specific player.

Flow:

1. Select the game.
2. Use `PLAY <call>` to invite a player.
3. The recipient gets an immediate readable invitation.
4. If there is one pending invitation, `A` accepts and `D` declines.
5. If there are many, the `INVITES` screen shows a numbered list.

Technical `ACCEPT <id>` and `DECLINE <id>` commands remain available for
compatibility.

## Room flow

`ROOM` games use a shared room rather than a direct one-to-one challenge.

Flow:

1. Select the game.
2. Use `CREATE` to create a room or `JOIN <id>` to join one.
3. The host sees the current room state and can start the game.
4. `START` is allowed only after `min_players` is reached.
5. `LEAVE` or `BACK` returns the player to the lobby.

The current implementation uses the simpler stable rule that if the host leaves,
the room closes and all players return to the lobby.

## Tic-Tac-Toe reference game

Tic-Tac-Toe is the reference game used to exercise the framework.

Metadata:

- `min_players = 2`
- `max_players = 2`
- `join_mode = INVITE`

Rules:

- two players
- X and O
- alternating moves
- win after 3 marks in a line
- draw when the board is full
- the server validates every move

At the start of the game the client gets a short ASCII description and a small
board. `HELP` is available on demand. After a result, `R` requests a rematch
and `Q` returns to the lobby.

## ASCII and CRLF

Game Hall uses plain ASCII as the baseline interface:

- 7-bit ASCII as the lowest common denominator
- no required Unicode
- no required colors
- no required ANSI
- no emoji
- roughly 80 columns or less

CRLF handling is normalized inside the game hall renderer so the service does
not produce `\r\r\n` style double conversion. New views may start with a CRLF
separator when that helps separate them from the previous prompt.

## Phrase packs

Phrase packs are not attached to a specific game. They are a shared Hall
resource for future word games.

Planned shape:

- `PhrasePack`
  - `ID`
  - `name`
  - `language`
  - `version`
  - `entries`
- `Entry`
  - `category`
  - `phrase`

That allows Hangman, word games and similar features to use the same source of
phrases without duplicating data.

## How to add a new game

1. Implement the `Game` interface.
2. Keep the authoritative game state inside the implementation.
3. Expose a compact `PlayerView`.
4. Register a `GameDefinition` with the correct join mode and player limits.
5. Add the localized name and the short ASCII instructions.
6. Add tests for move validation, end state and player flow.

## Current limits

- Session state is in memory.
- Rooms are in memory.
- Invitations are in memory.
- There is no persistence or reconnection.
- There is no ranking, tournament or bot support.
- GUI support is secondary to the terminal flow.
- Future games such as Battleships, Chess and Checkers are only prepared for by
  the architecture, not implemented now.
