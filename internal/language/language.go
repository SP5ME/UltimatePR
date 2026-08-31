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
	"callsign": "Znak: ", "invalid_call": "Nieprawidlowy znak.\r\n", "db_error": "Blad bazy danych.\r\n", "hello_bbs": "Witaj %s. Wpisz H, aby zobaczyc pomoc.\r\n",
	"bbs_help":    "H pomoc | L lista | I informacje | LM lista | LN/N nowe | LS wyslane | LB biuletyny | R <id> czytaj | RE <id> odpowiedz | S/SP <znak@BBS> prywatna | SB <obszar> biuletyn | ST <znak@BBS> NTS | FS <id> forwarding | K <id> usun | PROFILE | NAME | HOMEBBS | QTH | LOC | CONV | LANG PL/EN | B powrot\r\n",
	"no_messages": "Brak wiadomosci.\r\n", "no_new": "Brak nowych wiadomosci.\r\n", "usage_read": "Uzycie: R <id>\r\n", "invalid_id": "Nieprawidlowy numer.\r\n", "error": "Blad: %v\r\n", "usage_kill": "Uzycie: K <id>\r\n", "deleted": "Usunieto.\r\n", "usage_send": "Uzycie: S <znak lub znak@BBS>\r\n", "unknown": "Nieznane polecenie. Wpisz H.\r\n", "subject": "Temat: ", "body": "Wpisz tresc. Zakoncz linia /EX.\r\n", "saved": "Wiadomosc #%d zostala zapisana.\r\n", "return_node": "Powrot do noda.\r\n", "lang_set": "Jezyk ustawiony na polski.\r\n", "lang_usage": "Uzycie: LANG PL albo LANG EN\r\n", "reply_subject": "Odp: %s", "forward_none": "Wiadomosc nie jest w kolejce forwardingu.\r\n", "forward_line": "%-12s stan=%-9s proby=%d blad=%s\r\n", "usage_reply": "Uzycie: RE <id>\r\n", "usage_forward": "Uzycie: FS <id>\r\n",
	"node_call": "Znak: ", "hello_node": "Witaj %s. Wpisz ?, aby zobaczyc pomoc.\r\n", "node_help": "NODES | ROUTES | PORTS | SERVICES | BBS | C <usluga/znak> | LANG PL/EN | BYE\r\n", "no_neighbors": "Brak aktywnych sasiadow.\r\n", "no_routes": "Brak aktywnych tras.\r\n", "bbs_unavailable": "BBS jest niedostepny.\r\n", "usage_connect": "Uzycie: C <usluga/znak>\r\n", "no_route": "Brak trasy do %s.\r\n", "route_found": "Trasa %s przez %s, port %s, jakosc %d. Rzeczywiste lacze bedzie dodane w kolejnym etapie.\r\n", "returned": "Powrot do noda.\r\n", "lang_pl": "Jezyk ustawiony na polski.\r\n", "lang_en": "Language changed to English.\r\n",
}
var en = map[string]string{
	"callsign": "Callsign: ", "invalid_call": "Invalid callsign.\r\n", "db_error": "Database error.\r\n", "hello_bbs": "Hello %s. Type H for help.\r\n", "bbs_help": "H help | L list | I information | LM list | LN/N new | LS sent | LB bulletins | R <id> read | RE <id> reply | S/SP <call@BBS> private | SB <area> bulletin | ST <call@BBS> NTS | FS <id> forwarding | K <id> delete | PROFILE | NAME | HOMEBBS | QTH | LOC | CONV | LANG PL/EN | B return\r\n", "no_messages": "No messages.\r\n", "no_new": "No new messages.\r\n", "usage_read": "Usage: R <id>\r\n", "invalid_id": "Invalid message number.\r\n", "error": "Error: %v\r\n", "usage_kill": "Usage: K <id>\r\n", "deleted": "Deleted.\r\n", "usage_send": "Usage: SP <call or call@BBS>\r\n", "unknown": "Unknown command. Type H.\r\n", "subject": "Subject: ", "body": "Enter text. Finish with /EX on a separate line.\r\n", "saved": "Message #%d saved.\r\n", "return_node": "Return to node.\r\n", "lang_set": "Language changed to English.\r\n", "lang_usage": "Usage: LANG PL or LANG EN\r\n", "reply_subject": "Re: %s", "forward_none": "Message is not queued for forwarding.\r\n", "forward_line": "%-12s status=%-9s attempts=%d error=%s\r\n", "usage_reply": "Usage: RE <id>\r\n", "usage_forward": "Usage: FS <id>\r\n",
	"node_call": "Callsign: ", "hello_node": "Hello %s. Type ? for help.\r\n", "node_help": "NODES | ROUTES | PORTS | SERVICES | BBS | C <service/call> | LANG PL/EN | BYE\r\n", "no_neighbors": "No active neighbors.\r\n", "no_routes": "No active routes.\r\n", "bbs_unavailable": "BBS unavailable.\r\n", "usage_connect": "Usage: C <service/callsign>\r\n", "no_route": "No route to %s.\r\n", "route_found": "Route %s via %s on %s, quality %d. Live link connection is the next protocol stage.\r\n", "returned": "Returned to node.\r\n", "lang_pl": "Jezyk ustawiony na polski.\r\n", "lang_en": "Language changed to English.\r\n",
}
