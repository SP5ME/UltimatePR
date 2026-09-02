package language

import "strings"

func Normalize(v string) string {
	if strings.EqualFold(strings.TrimSpace(v), "en") {
		return "en"
	}
	return "pl"
}
func T(lang, key string) string {
	if Normalize(lang) == "en" {
		if v, ok := en[key]; ok {
			return v
		}
	}
	if v, ok := pl[key]; ok {
		return v
	}
	return key
}

var polishASCII = strings.NewReplacer("ą", "a", "ć", "c", "ę", "e", "ł", "l", "ń", "n", "ó", "o", "ś", "s", "ź", "z", "ż", "z", "Ą", "A", "Ć", "C", "Ę", "E", "Ł", "L", "Ń", "N", "Ó", "O", "Ś", "S", "Ź", "Z", "Ż", "Z")

func ASCII(v string) string { return polishASCII.Replace(v) }

var pl = map[string]string{
	"game_welcome": "\r\nSALON GIER\r\n", "game_lobby": "1 Gry\r\n2 Gracze\r\n3 Zaproszenia\r\n4 Pomoc\r\n5 Wyjscie\r\n", "game_games": "TIC-TAC-TOE - PLAY <znak>\r\n", "game_players_header": "GRACZE\r\n", "game_status_lobby": "Lobby", "game_status_waiting": "Oczekuje", "game_status_in_room": "Pokoj", "game_status_playing": "Gra", "game_no_players": "Brak graczy.\r\n", "game_no_invites": "Brak zaproszen.\r\n", "game_menu_header": "SALON GIER\r\n", "game_menu_footer": "GAMES lub numer - wybierz gre\r\nPLAYERS - gracze\r\nINVITES - zaproszenia\r\nHELP - pomoc\r\nQUIT - wyjscie\r\n", "game_selected_header": "GRA\r\n", "game_help": "GAMES | PLAYERS | INVITES | HELP | QUIT\r\n", "game_play_usage": "Uzycie: PLAY <znak>\r\n", "game_accept_usage": "Uzycie: A [id lub numer]\r\n", "game_decline_usage": "Uzycie: D [id lub numer]\r\n", "game_unknown": "Nieznane polecenie. Wpisz HELP.\r\n", "game_goodbye": "73!\r\n", "game_connect_error": "Nie mozna wejsc do Salonu Gier.\r\n", "game_invited": "%s zaprasza Cie do gry: %s.\r\nA - akceptuj\r\nD - odrzuc\r\n", "game_invite_sent": "Zaproszenie do %s wyslane do %s.\r\n", "game_invite_cancelled": "Zaproszenie do %s anulowane przez %s.\r\n", "game_invite_expired": "Zaproszenie do %s wygaslo.\r\n", "game_declined": "Zaproszenie do %s odrzucone.\r\n", "game_declined_self": "Odrzucono zaproszenie do %s.\r\n", "game_invites_header": "ZAPROSZENIA\r\n", "game_invites_footer": "A 1 - akceptuj\r\nD 1 - odrzuc\r\nBACK - lobby\r\n", "game_game_invite_help": "PLAY <znak>\r\nPLAYERS - gracze\r\nINVITES - zaproszenia\r\nBACK - lobby\r\n", "game_game_back": "BACK - lobby\r\n", "game_game_solo_help": "START - uruchom\r\nBACK - lobby\r\n", "game_room_intro": "OTWARTE GRY\r\nCREATE - utworz gre\r\nJOIN <id> - dolacz\r\nBACK - lobby\r\n", "game_rooms_header": "OTWARTE GRY\r\n", "game_rooms_columns": "ID   HOST     GRACZE\r\n", "game_rooms_footer": "CREATE - utworz gre\r\nJOIN <id> - dolacz\r\nBACK - lobby\r\n", "game_no_rooms": "Brak otwartych gier.\r\n", "game_room_created": "Utworzono pokoj #%s.\r\n", "game_room_join_usage": "Uzycie: JOIN <id>\r\n", "game_room_help": "CREATE | JOIN <id> | BACK\r\n", "game_room_active_help": "START | LEAVE | BACK\r\n", "game_room_state_header": "POKOJ\r\n", "game_room_host": "Host", "game_room_players": "Gracze", "game_session_help": "BOARD | REMATCH | QUIT\r\n", "game_started": "Gra %s rozpoczeta.\r\n", "game_turn": "Ruch: %s\r\n", "game_draw": "Remis.\r\n", "game_winner": "Wygrywa %s.\r\n", "game_finished_help": "R - rewanz\r\nQ - lobby\r\n", "game_rematch_request": "%s proponuje rewanz. Wpisz REMATCH, aby zaakceptowac.\r\n", "game_rematch_started": "Rewanz rozpoczety.\r\n", "game_back_lobby": "Powrot do lobby.\r\n", "game_left": "%s opuscil gre.\r\n", "game_disconnected": "%s rozlaczyl sie. Gra przerwana.\r\n", "game_invalid": "Nieprawidlowa akcja.\r\n", "game_not_turn": "To nie Twoj ruch.\r\n", "game_occupied": "Pole jest zajete.\r\n", "game_already_finished": "Gra jest zakonczona.\r\n", "game_tictactoe_name": "Kółko i krzyżyk", "ttt_name": "Kółko i krzyżyk", "ttt_intro": "Uloz 3 swoje znaki w jednej linii.\r\nRuch wykonujesz wpisujac wspolrzedne pola, np. B2.\r\n", "ttt_controls": "1 2 3\r\nA . . .\r\nB . . .\r\nC . . .\r\n", "ttt_help": "KOLKO I KRZYZYK\r\nUloz 3 swoje znaki w jednej linii.\r\nRuch: wpisz wspolrzedne, np. B2.\r\nBOARD - pokaz plansze\r\nHELP - pomoc\r\nQUIT - wyjdz\r\n",
	"callsign": "Znak: ", "invalid_call": "Nieprawidlowy znak.\r\n", "db_error": "Blad bazy danych.\r\n", "hello_bbs": "Witaj %s. Wpisz H, aby zobaczyc pomoc.\r\n",
	"bbs_help":    "H pomoc | L lista | I informacje | LM lista | LN/N nowe | LS wyslane | LB biuletyny | R <id> czytaj | RE <id> odpowiedz | S/SP <znak@BBS> prywatna | SB <obszar> biuletyn | ST <znak@BBS> NTS | FS <id> forwarding | K <id> usun | PROFILE | NAME | HOMEBBS | QTH | LOC | CONV | LANG PL/EN | B powrot\r\n",
	"no_messages": "Brak wiadomosci.\r\n", "no_new": "Brak nowych wiadomosci.\r\n", "usage_read": "Uzycie: R <id>\r\n", "invalid_id": "Nieprawidlowy numer.\r\n", "error": "Blad: %v\r\n", "usage_kill": "Uzycie: K <id>\r\n", "deleted": "Usunieto.\r\n", "usage_send": "Uzycie: S <znak lub znak@BBS>\r\n", "unknown": "Nieznane polecenie. Wpisz H.\r\n", "subject": "Temat: ", "body": "Wpisz tresc. Zakoncz linia /EX.\r\n", "saved": "Wiadomosc #%d zostala zapisana.\r\n", "return_node": "Powrot do noda.\r\n", "lang_set": "Jezyk ustawiony na polski.\r\n", "lang_usage": "Uzycie: LANG PL albo LANG EN\r\n", "reply_subject": "Odp: %s", "forward_none": "Wiadomosc nie jest w kolejce forwardingu.\r\n", "forward_line": "%-12s stan=%-9s proby=%d blad=%s\r\n", "usage_reply": "Uzycie: RE <id>\r\n", "usage_forward": "Uzycie: FS <id>\r\n",
	"node_call": "Znak: ", "hello_node": "Witaj %s. Wpisz ?, aby zobaczyc pomoc.\r\n", "node_help": "NODES | ROUTES | PORTS | SERVICES | BBS | C <usluga/znak> | LANG PL/EN | BYE\r\n", "no_neighbors": "Brak aktywnych sasiadow.\r\n", "no_routes": "Brak aktywnych tras.\r\n", "bbs_unavailable": "BBS jest niedostepny.\r\n", "usage_connect": "Uzycie: C <usluga/znak>\r\n", "no_route": "Brak trasy do %s.\r\n", "route_found": "Trasa %s przez %s, port %s, jakosc %d. Rzeczywiste lacze bedzie dodane w kolejnym etapie.\r\n", "returned": "Powrot do noda.\r\n", "lang_pl": "Jezyk ustawiony na polski.\r\n", "lang_en": "Language changed to English.\r\n",
}
var en = map[string]string{
	"game_welcome": "\r\nGAME HALL\r\n", "game_lobby": "1 Games\r\n2 Players\r\n3 Invitations\r\n4 Help\r\n5 Exit\r\n", "game_games": "TIC-TAC-TOE - PLAY <callsign>\r\n", "game_players_header": "PLAYERS\r\n", "game_status_lobby": "Lobby", "game_status_waiting": "Waiting", "game_status_in_room": "In room", "game_status_playing": "Playing", "game_no_players": "No players.\r\n", "game_no_invites": "No invitations.\r\n", "game_menu_header": "GAME HALL\r\n", "game_menu_footer": "GAMES or a number - choose a game\r\nPLAYERS - players\r\nINVITES - invitations\r\nHELP - help\r\nQUIT - exit\r\n", "game_selected_header": "GAME\r\n", "game_help": "GAMES | PLAYERS | INVITES | HELP | QUIT\r\n", "game_play_usage": "Usage: PLAY <call>\r\n", "game_accept_usage": "Usage: A [id or number]\r\n", "game_decline_usage": "Usage: D [id or number]\r\n", "game_unknown": "Unknown command. Type HELP.\r\n", "game_goodbye": "73!\r\n", "game_connect_error": "Cannot enter the Game Hall.\r\n", "game_invited": "%s invites you to play: %s.\r\nA - accept\r\nD - decline\r\n", "game_invite_sent": "Invitation to %s sent to %s.\r\n", "game_invite_cancelled": "Invitation to %s cancelled by %s.\r\n", "game_invite_expired": "Invitation to %s expired.\r\n", "game_declined": "Invitation to %s declined.\r\n", "game_declined_self": "You declined the invitation to %s.\r\n", "game_invites_header": "INVITATIONS\r\n", "game_invites_footer": "A 1 - accept\r\nD 1 - decline\r\nBACK - lobby\r\n", "game_game_invite_help": "PLAY <call>\r\nPLAYERS - players\r\nINVITES - invitations\r\nBACK - lobby\r\n", "game_game_back": "BACK - lobby\r\n", "game_game_solo_help": "START - begin\r\nBACK - lobby\r\n", "game_room_intro": "OPEN GAMES\r\nCREATE - create a game\r\nJOIN <id> - join\r\nBACK - lobby\r\n", "game_rooms_header": "OPEN GAMES\r\n", "game_rooms_columns": "ID   HOST     PLAYERS\r\n", "game_rooms_footer": "CREATE - create a game\r\nJOIN <id> - join\r\nBACK - lobby\r\n", "game_no_rooms": "No open games.\r\n", "game_room_created": "Room #%s created.\r\n", "game_room_join_usage": "Usage: JOIN <id>\r\n", "game_room_help": "CREATE | JOIN <id> | BACK\r\n", "game_room_active_help": "START | LEAVE | BACK\r\n", "game_room_state_header": "ROOM\r\n", "game_room_host": "Host", "game_room_players": "Players", "game_session_help": "BOARD | REMATCH | QUIT\r\n", "game_started": "Game %s started.\r\n", "game_turn": "Turn: %s\r\n", "game_draw": "Draw.\r\n", "game_winner": "%s wins.\r\n", "game_finished_help": "R - rematch\r\nQ - lobby\r\n", "game_rematch_request": "%s requests a rematch. Type REMATCH to accept.\r\n", "game_rematch_started": "Rematch started.\r\n", "game_back_lobby": "Back to lobby.\r\n", "game_left": "%s left the game.\r\n", "game_disconnected": "%s disconnected. Game interrupted.\r\n", "game_invalid": "Invalid action.\r\n", "game_not_turn": "It is not your turn.\r\n", "game_occupied": "Field is occupied.\r\n", "game_already_finished": "Game is finished.\r\n", "ttt_name": "Tic-Tac-Toe", "ttt_intro": "Make 3 of your marks in a line.\r\nEnter a coordinate such as B2.\r\n", "ttt_controls": "1 2 3\r\nA . . .\r\nB . . .\r\nC . . .\r\n", "ttt_help": "TIC-TAC-TOE\r\nGoal: make a line of 3 marks.\r\nMove: enter coordinates, for example B2.\r\nBOARD - show board\r\nHELP - help\r\nQUIT - leave\r\n",
	"callsign": "Callsign: ", "invalid_call": "Invalid callsign.\r\n", "db_error": "Database error.\r\n", "hello_bbs": "Hello %s. Type H for help.\r\n", "bbs_help": "H help | L list | I information | LM list | LN/N new | LS sent | LB bulletins | R <id> read | RE <id> reply | S/SP <call@BBS> private | SB <area> bulletin | ST <call@BBS> NTS | FS <id> forwarding | K <id> delete | PROFILE | NAME | HOMEBBS | QTH | LOC | CONV | LANG PL/EN | B return\r\n", "no_messages": "No messages.\r\n", "no_new": "No new messages.\r\n", "usage_read": "Usage: R <id>\r\n", "invalid_id": "Invalid message number.\r\n", "error": "Error: %v\r\n", "usage_kill": "Usage: K <id>\r\n", "deleted": "Deleted.\r\n", "usage_send": "Usage: SP <call or call@BBS>\r\n", "unknown": "Unknown command. Type H.\r\n", "subject": "Subject: ", "body": "Enter text. Finish with /EX on a separate line.\r\n", "saved": "Message #%d saved.\r\n", "return_node": "Return to node.\r\n", "lang_set": "Language changed to English.\r\n", "lang_usage": "Usage: LANG PL or LANG EN\r\n", "reply_subject": "Re: %s", "forward_none": "Message is not queued for forwarding.\r\n", "forward_line": "%-12s status=%-9s attempts=%d error=%s\r\n", "usage_reply": "Usage: RE <id>\r\n", "usage_forward": "Usage: FS <id>\r\n",
	"node_call": "Callsign: ", "hello_node": "Hello %s. Type ? for help.\r\n", "node_help": "NODES | ROUTES | PORTS | SERVICES | BBS | C <service/call> | LANG PL/EN | BYE\r\n", "no_neighbors": "No active neighbors.\r\n", "no_routes": "No active routes.\r\n", "bbs_unavailable": "BBS unavailable.\r\n", "usage_connect": "Usage: C <service/callsign>\r\n", "no_route": "No route to %s.\r\n", "route_found": "Route %s via %s on %s, quality %d. Live link connection is the next protocol stage.\r\n", "returned": "Returned to node.\r\n", "lang_pl": "Jezyk ustawiony na polski.\r\n", "lang_en": "Language changed to English.\r\n",
}

func init() {
	// Keep menu labels defined in one final override block.
	pl["game_menu_footer"] = "1. Gry\r\n2. Gracze\r\n3. Zaproszenia\r\n4. Pomoc\r\n5. Wyjscie\r\n"
	pl["game_games_footer"] = ""
	pl["game_back_option"] = "%d. Powrot\r\n"
	pl["game_invites_footer"] = "A 1 - akceptuj\r\nD 1 - odrzuc\r\n"
	pl["game_game_invite_help"] = "1. Zapros gracza\r\n2. Gracze\r\n3. Zaproszenia\r\n4. Powrot\r\n"
	pl["game_invite_target"] = "Podaj znak gracza:\r\n"
	pl["game_players_help"] = "Wpisz numer powrotu.\r\nBACK - lobby\r\n"
	en["game_menu_footer"] = "1. Games\r\n2. Players\r\n3. Invitations\r\n4. Help\r\n5. Exit\r\n"
	en["game_games_footer"] = ""
	en["game_back_option"] = "%d. Back\r\n"
	en["game_invites_footer"] = "A 1 - accept\r\nD 1 - decline\r\n"
	en["game_game_invite_help"] = "1. Invite a player\r\n2. Players\r\n3. Invitations\r\n4. Back\r\n"
	en["game_invite_target"] = "Enter player callsign:\r\n"
	en["game_players_help"] = "Enter the back option number.\r\nBACK - lobby\r\n"
	pl["game_games_header"] = "GRY\r\n\r\n"
	pl["game_games_footer"] = ""
	pl["game_games_help"] = "Wpisz numer gry.\r\nBACK - lobby\r\n"
	pl["game_player_count"] = "%d graczy"
	pl["game_player_range"] = "%d-%d graczy"
	pl["ttt_goal"] = "Gra dla 2 graczy.\r\nUloz 3 swoje znaki w jednej linii.\r\n\r\n"
	pl["ttt_move_help"] = "Ruch: wpisz wspolrzedne pola, np. B2.\r\n\r\n"
	pl["ttt_heading"] = "KOLKO I KRZYZYK"
	pl["game_connect4_name"] = "Connect Four"
	pl["game_hangman_name"] = "Wisielec"
	pl["game_word_name"] = "Haslo"
	en["game_menu_footer"] = "1. Games\r\n2. Players\r\n3. Invitations\r\n4. Help\r\n5. Exit\r\n"
	en["game_games_header"] = "GAMES\r\n\r\n"
	en["game_games_footer"] = ""
	en["game_games_help"] = "Enter a game number.\r\nBACK - lobby\r\n"
	en["game_player_count"] = "%d players"
	en["game_player_range"] = "%d-%d players"
	en["ttt_goal"] = "Game for 2 players.\r\nMake 3 of your marks in a line.\r\n\r\n"
	en["ttt_move_help"] = "Move: enter coordinates, for example B2.\r\n\r\n"
	en["ttt_heading"] = "TIC-TAC-TOE"
	en["game_connect4_name"] = "Connect Four"
	en["game_hangman_name"] = "Hangman"
	en["game_word_name"] = "Word Game"
	pl["connect4_intro"] = "Gra dla 2 graczy.\r\nUloz 4 swoje pionki w jednej linii.\r\nRuch: wpisz numer kolumny 1-7.\r\n\r\n"
	pl["connect4_help"] = "BOARD - pokaz plansze\r\nHELP - pomoc\r\nQUIT - wyjdz\r\n"
	en["connect4_intro"] = "Game for 2 players.\r\nMake 4 of your pieces in a line.\r\nMove: enter a column number 1-7.\r\n\r\n"
	en["connect4_help"] = "BOARD - show board\r\nHELP - help\r\nQUIT - leave\r\n"
	pl["game_secret_intro"] = "Gra dla 1-6 graczy.\r\n"
	en["game_secret_intro"] = "Game for 1-6 players.\r\n"
	pl["game_secret_help"] = "1. Gra solo\r\n2. Utworz pokoj\r\n3. Otwarte gry\r\n4. Powrot\r\n"
	pl["word_intro"] = "HASLO\r\n\r\n"
	pl["word_state"] = "Kategoria: %s\r\n\r\n%s\r\n\r\nUzyte: %s\r\nRuch: %s\r\n"
	pl["hangman_intro"] = "WISIELEC\r\n\r\n"
	pl["hangman_state"] = "Kategoria: %s\r\n%s\r\n\r\nUzyte: %s\r\nBledy: %d/6\r\nRuch: %s\r\n"
	en["game_secret_help"] = "1. Solo game\r\n2. Create room\r\n3. Open games\r\n4. Back\r\n"
	en["word_intro"] = "WORD GAME\r\n\r\n"
	en["word_state"] = "Category: %s\r\n\r\n%s\r\n\r\nUsed: %s\r\nTurn: %s\r\n"
	en["hangman_intro"] = "HANGMAN\r\n\r\n"
	en["hangman_state"] = "Category: %s\r\n%s\r\n\r\nUsed: %s\r\nErrors: %d/6\r\nTurn: %s\r\n"
	pl["game_tictactoe_name"] = "Kolko i krzyzyk"
	pl["ttt_name"] = "Kolko i krzyzyk"
}
