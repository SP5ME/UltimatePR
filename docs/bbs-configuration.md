# Konfiguracja BBS

Panel `Konfiguracja -> BBS` opisuje ustawienia używane przez aktualny runtime
UltimatePR. Poniższa tabela zbiera pola `bbs.*`, `bbs.forwarding.*` oraz
`bbs.forwarding.peers[].*` razem z ich realnym wpływem na działanie programu.

## Ustawienia BBS

| Opcja | Znaczenie | Zakres / jednostka | Domyślnie | Wpływ na runtime |
| --- | --- | --- | --- | --- |
| `bbs.enabled` | Uruchamia usługę BBS | wl./wyl. | `false` w trybie `station` | Działa |
| `bbs.listen` | Lokalny listener TCP dla BBS | `host:port` | `127.0.0.1:8023` | Działa |
| `bbs.forward_listen` | Listener TCP dla sesji BBS forwarding | `host:port` | `127.0.0.1:8024` | Aktywne, gdy włączony jest forwarding |
| `bbs.database` | Plik bazy wiadomości, profili i White Pages | ścieżka pliku | `/var/lib/ultimatepr/bbs.json` | Działa |
| `bbs.title` | Tytuł pokazywany w sesji BBS | tekst | `<CALL> BBS` | Działa |
| `bbs.callsign` | Callsign BBS | callsign 1-6 znaków | callsign stacji | Działa |
| `bbs.ssid` | SSID BBS | `0..15` | `8` | Działa |
| `bbs.sysop_callsign` | Callsign sysopa w komunikatach i info | callsign | `bbs.callsign` | Działa |
| `bbs.hierarchical_address` | Adres hierarchiczny TAPR używany w forwardingu | `BBS.[#AREA.][REGION.]COUNTRY.CONTINENT` | brak w trybie bez forwardingu | Działa; wymagany przy `bbs.forwarding.enabled=true` |
| `bbs.language` | Język sesji BBS | `pl` / `en` | język aplikacji | Działa |
| `bbs.max_sessions` | Maksymalna liczba równoczesnych sesji | liczba | `10` | Działa |
| `bbs.beacon_via` | Trasa VIA / DIGI dla beacona BBS | lista callsignów po przecinku | puste | Działa |
| `bbs.welcome_message` | Powitanie sesji BBS | tekst | komunikat wbudowany | Działa |
| `bbs.new_user_message` | Komunikat przy pierwszym logowaniu | tekst | komunikat wbudowany | Działa |
| `bbs.info_message` | Komunikat `INFO` | tekst | komunikat wbudowany | Działa |
| `bbs.prompt` | Prompt sesji | tekst | `{CALL}> ` | Działa |
| `bbs.goodbye_message` | Pożegnanie sesji | tekst | komunikat wbudowany | Działa |

### Ustawienia housekeeping

| Opcja | Znaczenie | Zakres / jednostka | Domyślnie | Wpływ na runtime |
| --- | --- | --- | --- | --- |
| `bbs.housekeeping.bulletin_retention_days` | Retencja biuletynów | dni | `90` | Aktywne |
| `bbs.housekeeping.personal_retention_days` | Retencja wiadomości prywatnych i traffic | dni | `180` | Aktywne |
| `bbs.housekeeping.log_retention_days` | Zarezerwowana retencja logów BBS | dni | `30` | Nieaktywne runtime |

### Forwarding globalny

| Opcja | Znaczenie | Zakres / jednostka | Domyślnie | Wpływ na runtime |
| --- | --- | --- | --- | --- |
| `bbs.forwarding.enabled` | Włącza planner i forwarder | wl./wyl. | `false` | Działa |
| `bbs.forwarding.interval_minutes` | Interwał planowania kolejek i prób forwardingu | minuty | `15` | Działa |
| `bbs.forwarding.connect_timeout_seconds` | Timeout połączenia z peerem | sekundy | `15` | Działa |
| `bbs.forwarding.session_timeout_seconds` | Timeout całej sesji forwardingu | sekundy | `120` | Działa |
| `bbs.forwarding.max_messages_per_session` | Limit wiadomości na peer i sesję planowania | liczba | `50` | Działa |
| `bbs.forwarding.max_body_bytes` | Limit rozmiaru treści wiadomości | bajty | `131072` | Aktywne dla sesji lokalnych, importu i odbioru forwardingowego |

### Peer BBS

| Opcja | Znaczenie | Zakres / jednostka | Domyślnie | Wpływ na runtime |
| --- | --- | --- | --- | --- |
| `bbs.forwarding.peers[].id` | Stabilny identyfikator peera | niepusty tekst | brak | Działa |
| `bbs.forwarding.peers[].callsign` | Callsign peer BBS | callsign | brak | Działa |
| `bbs.forwarding.peers[].ssid` | SSID peer BBS | `0..15` | `0` jeśli nie podano | Działa |
| `bbs.forwarding.peers[].hierarchical_address` | TAPR address peera | format TAPR | brak | Działa |
| `bbs.forwarding.peers[].enabled` | Włącza lub wyłącza peera | wl./wyl. | `true` po stronie runtime, jeśli pole nie jest wyłączone | Działa |
| `bbs.forwarding.peers[].send` | Zezwala na wysyłkę do peera | wl./wyl. | brak, czyli domyślnie `true` | Działa |
| `bbs.forwarding.peers[].receive` | Zezwala na odbiór od peera | wl./wyl. | brak, czyli domyślnie `true` | Działa |
| `bbs.forwarding.peers[].transport` | Transport peera | `telnet` / `node` | `telnet` w praktyce konfiguracji | Częściowo aktywne: aktywny dialer outbound używa `telnet`; `node` jest tylko parsowany i akceptowany przez walidację |
| `bbs.forwarding.peers[].host` | Host TCP dla `telnet` | host | brak | Działa |
| `bbs.forwarding.peers[].port` | Port TCP dla `telnet` | `1..65535` | `0` | Działa |
| `bbs.forwarding.peers[].via_node` | Opcjonalna trasa przez node | tekst | brak | Nieaktywne runtime; wartość jest przenoszona do modelu, ale nieużywana przez forwarding |
| `bbs.forwarding.peers[].schedule` | Okna aktywności `HH:MM-HH:MM` | lista tekstowa | pusta, czyli zawsze | Aktywne; czas lokalny hosta |
| `bbs.forwarding.peers[].private_routes` | Reguły prywatnych tras | lista wzorców | brak | Działa |
| `bbs.forwarding.peers[].bulletin_scopes` | Reguły zakresu bulletinów | lista designatorów lub `*` | brak | Działa |
| `bbs.forwarding.peers[].to_calls` | Dopasowanie po odbiorcy `TO` | lista callsignów | brak | Działa |
| `bbs.forwarding.peers[].at_calls` | Dopasowanie po pierwszym BBS w `@` | lista callsignów | brak | Działa |
| `bbs.forwarding.peers[].hierarchical_routes` | Dopasowanie po pełnym adresie TAPR | lista tras TAPR | brak | Działa |

## Zachowanie runtime

- `bbs.enabled` uruchamia lokalną usługę BBS na `bbs.listen`.
- `bbs.forwarding.enabled` dodatkowo uruchamia planner kolejek, forwarder
  outbound i listener sesji BBS na `bbs.forward_listen`.
- `bbs.beacon_via` wpływa na trasę VIA / DIGI dla beacona BBS.
- `bbs.sysop_callsign` domyślnie przyjmuje `bbs.callsign`, jeśli pole jest puste.
- `bbs.max_sessions` ogranicza liczbę równoczesnych sesji BBS.
- `bbs.new_user_message`, `bbs.info_message`, `bbs.prompt` i
  `bbs.goodbye_message` są rozwijane z makrami w runtime.
- `send` / `receive` sterują kierunkiem obsługi peera. Brak pola oznacza tryb
  domyślny `true`.
- `via_node` jest zachowane w modelu, ale nie jest aktywnym elementem routingu;
  `schedule` ogranicza próby outbound do skonfigurowanych okien.
- Housekeeping uruchamia się przy starcie BBS i co 24 godziny. Usuwa wygasłe
  `B` oraz `P`/`T`, ale zachowuje wiadomości z niedostarczonym forwardingiem.
- `max_body_bytes` jest sprawdzany przed zapisem; zbyt duży payload forwardingowy
  kończy sesję błędem i nie jest zapisywany jako poprawna wiadomość.

## Service Registry / lokalne uslugi

BBS rejestruje sie w runtime pod `bbs.service_id` i jest potem rozpoznawany
przez wspolny Service Registry, zamiast byc wiazany wylacznie po callsign.

- `service_id` jest wewnetrznym identyfikatorem uslugi i pozostaje stabilny.
- `callsign` + `ssid` sa adresem AX.25 widocznym dla uzytkownika i sieci.
- Gdy BBS jest aktywny lokalnie, resolver wybiera lokalny endpoint i nie wysyla
  ruchu na RF tylko dlatego, ze adres wyglada tak samo.
- Gdy BBS jest wylaczony, routing zwraca `service unavailable` zamiast
  probowac niejawnego fallbacku na radio.
- Duplikat `service_id` lub kolizja adresu z inna aktywna usluga jest bledem
  konfiguracji.

`transport: node` otwiera sesje przez lokalny `node-main` i istniejący Hub.
`via_node` pozostaje konkretnym zewnętrznym NODE/hopem. Wspólne wpisy
`remote_endpoints` można wskazywać przez `endpoint_id`; obsługiwane typy to
`ax25`, `node` i `tcp`. RF fallback wymaga jawnego `fallback_to_rf: true`.

Przykład endpointu NODE i peera wskazującego go bez powielania trasy:

```yaml
remote_endpoints:
  - id: sr5bbs
    callsign: SR5BBS-8
    transport: node
    via_node: SR5NODE-7
    enabled: true
bbs:
  forwarding:
    peers:
      - id: sr5bbs
        endpoint_id: sr5bbs
        enabled: true
```

Jawny fallback wymaga istniejącego portu radiowego:

```yaml
fallback_to_rf: true
rf_port: radio1
```

Panel WWW udostępnia odczytowe endpointy `/api/service-registry`,
`/api/remote-endpoints` oraz `/api/service-router/explain?target=...`.
Wyjaśnienie trasy nie zestawia połączenia.

## Reverse forwarding

Po zakończeniu części master wymiany strona inicjująca wysyła `F>`, a druga
strona może przedstawić własne propozycje w tej samej sesji TCP. UltimatePR
obsługuje ten etap dla aktywnego peera, ale nie otwiera w tym celu osobnego
połączenia. Jest to opis zachowania implementacji, nie deklaracja pełnej
interoperacyjności z zewnętrznymi BBS-ami.

## Granice zgodności

Runtime implementuje sprawdzony w testach projektu zakres obsługi BBS, adresowania
hierarchicznego i wymiany forwardingowej. Repozytorium nie zawiera testu
interoperacyjnego UltimatePR z LinBPQ ani z inną zewnętrzną implementacją BBS;
zgodność z zewnętrznymi systemami wymaga osobnych testów.

## Źródła

- [TAPR FTP Archive](https://tapr.org/ftp-archive/)
- [TAPR Bbssig / Recommendations](https://tapr.org/ftp-archive/?drawer=bbssig*recommendations)
- W archiwum TAPR w tym katalogu znajdują się między innymi materiały:
  - `Bbs Spec`
  - `Hierarchical`
  - `ISO3166`
  - `UKBBS`
- [Kod runtime BBS](../internal/bbs)
- [Konfiguracja runtime](../internal/config)
- [Uruchomienie serwera](../cmd/server/main.go)
