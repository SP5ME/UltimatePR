# Changelog

## 0.4.187-dev+7f80bad

### Added

- Dodano binarny bajt statusu operatora do ramek UPRD oraz obsługę jego interpretacji w MHEARD.
- Dodano zdalną komendę `/V`, zwracającą wersję aplikacji UltimatePR.
- Dodano eksperymentalną usługę IA korzystającą z Ollamy, dostępną jako osobna usługa NODE i bezpośredni znak AX.25, z ograniczeniem czasu, długości odpowiedzi, współbieżności oraz kolejki.
- Dodano wykrywanie modeli Ollamy z panelu konfiguracji oraz wybór modelu bez ręcznego przepisywania jego nazwy.
- Dodano edytowalne komunikaty powitania, przetwarzania i pożegnania dla usługi IA oraz osobne komunikaty sesji dla BBS i NODE.
- Dodano sekcję `Usługi`, wspólną obsługę eksperymentalnych przełączników usług oraz widoczny status IA w panelu WWW.
- Dodano okno pomocy MHEARD z opisem oznaczeń stacji oraz ikoną `help-circle-outline`.
- Dodano lokalne połączenie z usługą IA bez wysyłania ramek radiowych, gdy terminal łączy się ze skonfigurowanym znakiem IA tej samej instancji UltimatePR.
- Dodano przycisk `Wyślij status` w konfiguracji UPRdirect, pozwalający jednorazowo nadać bieżący status UPRD na żądanie.

### Changed

- Aktualizacja aplikacji najpierw rozłącza wszystkie aktywne sesje terminala i uruchamia aktualizator dopiero po ich pomyślnym zamknięciu.
- Ujednolicono wygląd pozostałych zakładek konfiguracji ze zwijanymi kartami znanymi z zakładki `Usługi`, zachowując dotychczasowy podział, zawartość i funkcjonalność ustawień.
- Zmieniono wersjonowanie pakietów `main` i `dev` na `MAJOR.MINOR.BUILD-KANAŁ+SHA`; ostatni liczbowy człon rośnie z każdym wypchniętym commitem.
- Rozdzielono ręczny przycisk klasycznego beaconu od automatycznego UPRD; oba mechanizmy mogą działać niezależnie.
- Przeniesiono ustawienia beaconu do osobnej zakładki `Beacon`, a zamknięcie Historii rozszerza pasek nowego połączenia.
- UPRD jest domyślnie włączone i działa niezależnie od klasycznego beaconu; przycisk nagłówka wysyła wyłącznie klasyczny beacon.
- Dodano wysyłanie statusu przy otwarciu pierwszej i zamknięciu ostatniej sesji WWW, bez dodatkowej ramki przy przełączeniu aplikacji.
- Ograniczono panel WWW do jednej aktywnej sesji; nowe logowanie automatycznie unieważnia poprzednią aplikację.
- Karty konfiguracji usług można zwijać i rozwijać, a włączenie każdej usługi jest niezależne od otwarcia jej ustawień.
- Ujednolicono ikony panelu, wskaźnik obecności operatora oraz przyciski pomocy, zastępując tekstowe znaki odpowiadającymi ikonami MDI.
- Uproszczono mobilne otwieranie nowego połączenia i poprawiono responsywny układ pól połączenia, terminala oraz panelu MHEARD.
- MHEARD pokazuje teraz jedną wspólną listę bez przełącznika `Direct` / `UPRD`: nadawcy statusów UPRD są oznaczeni zieloną ramką, a stacje zgłoszone przez cudzy status UPRD — niebieską.
- Dla stacji pośrednich MHEARD pokazuje tylko krótkie oznaczenie `via`; pełna ścieżka digipeaterów pozostaje zachowana i jest używana podczas zestawiania połączenia.
- Pole `SESJE` zmieniono na `TERMINALE`; osiem kart terminali skaluje się do dostępnej szerokości, a na małych ekranach przewija poziomo.
- Tekstowy przycisk zamknięcia terminala zastąpiono wyrównaną do prawej ikoną `close-box-outline`.
- Po rozpoczęciu nowego połączenia pola znaku i `VIA / DIGI` są automatycznie czyszczone.
- Zmiana nazwy użytkownika panelu nie wymaga już jednoczesnej zmiany hasła.
- Pytania do IA mogą obejmować wiele wierszy i są kończone poleceniem `/EX`; pojedyncze `Q` kończy sesję tylko poza rozpoczętym pytaniem.
- Ujednolicono typografię całego panelu przy użyciu czytelnego kroju proporcjonalnego; krój stałoodstępowy pozostaje wyłącznie w treści wiadomości terminala, aby zachować układ ASCII-artów.

### Fixed

- Zakładka `Beacon` nie miga już przy otwieraniu konfiguracji wcześniejszą nazwą `Bikon`.
- Wiadomości terminala pokazują czas `GG:MM:SS`, automatyczne pożegnanie jest widoczne również u nadawcy, a pierwsza wiadomość po ponownym połączeniu rozpoczyna nową pozycję zamiast zostać doklejoną do wcześniejszego odbioru.
- Terminale nie tworzą już dwóch pozycji dla tego samego pełnego znaku wraz z SSID. Ponowne połączenie przychodzące przejmuje istniejący terminal i jest prawidłowo odbierane, zamiast zostać automatycznie rozłączone po 30 sekundach.
- Monitor ponownie pokazuje rzeczywistą zawartość ramek UPRD zamiast samego opisu statusu.
- Naprawiono automatyczne powitanie połączenia przychodzącego AX.25: payload jest kończony pojedynczym `CRLF` (`0x0D 0x0A`).
- MHEARD jest zachowywane podczas planowanego restartu aplikacji i przywracane wyłącznie dla nadal skonfigurowanych portów oraz nieprzeterminowanych wpisów.
- Poprawiono wykrywanie niezapisanych zmian konfiguracji po responsywnym przerysowaniu formularza oraz po wczytaniu wartości znormalizowanych przez serwer.
- Komunikat zapisu konfiguracji prawidłowo rozróżnia sprawdzanie ustawień, zapis i konieczność restartu.
- Przywrócono zgodne odbieranie standardowego kanału KISS oraz filtrowanie ramek właściwych dla skonfigurowanego portu.
- Ujednolicono odczyt poleceń BBS, NODE i IA dla zakończeń wiersza `CR`, `LF` oraz `CRLF`, używanych przez różne terminale Packet Radio.
- Ręczne i automatyczne wysyłanie UPRD respektuje teraz widoczny przełącznik `Włącz funkcje UPRdirect`; stara flaga eksperymentalna dotyczy wyłącznie opcjonalnej mapy i nie blokuje już statusów.
- Naprawiono eksperymentalny przełącznik mapy, który mógł być zapisany jako aktywny, ale nie udostępniał widoku z powodu odwołania do nieistniejącego nadrzędnego pola.
- Wyłączone UPRD nie próbuje już nadawać statusu po zmianie obecności operatora, a komunikat walidacji klasycznego beaconu prawidłowo podaje minimalny interwał jednej minuty.

## 2026-08-29 - terminal mobilny i praca stacji w tle

- Dodano uproszczony interfejs terminala dla telefonów i małych ekranów: zwarty układ połączenia i sesji, pole wiadomości dopasowane do obsługi dotykowej oraz wysuwany panel MHEARD.
- Stacja operatora pozostaje aktywna po zamknięciu panelu WWW i obsługuje w tle połączenia przychodzące oraz komendy `/I`, `/MH`, `/H` i `/?`.
- Dodano wykrywanie obecności operatora na podstawie otwartego panelu WWW oraz osobną, edytowalną wiadomość `Nieobecny`, wysyłaną zamiast zwykłego powitania podczas pracy w tle.
- Ujednolicono nazewnictwo konfiguracji: zakładka klasycznego beaconu nosi teraz standardową nazwę `Beacon`.
- Uproszczono nagłówek lewego panelu terminala do samej nazwy `HISTORIA`.
- Domyślna treść beaconu stacji to wyłącznie makro `{LOC}`; bez skonfigurowanego lokatora rozwija się do pustego tekstu. Obok edytora dodano przycisk `Przywróć domyślne`.

## 2026-08-28 - uporządkowanie konfiguracji i terminala

- Uporządkowano konfigurację serwera: kategorie są dostępne w pionowym pasku, a treść wybranej sekcji jest wyświetlana w osobnej kolumnie.
- Zmieniono układ terminala: pasek nowego połączenia zajmuje tylko środkową kolumnę, a Historia i MHEARD wypełniają całą wysokość obszaru roboczego.
- Przyciski `Połącz ponownie` i `Rozłącz` znajdują się w nagłówku aktywnego terminala, natomiast `Monitor` pozostaje przy liście sesji.
- Przycisk `Beacon` opisano jako `Wyślij beacon`, a przełącznik `Direct` / `UPRD` wyrównano w panelu MHEARD.

## 2026-08-27 - poprawki mapy, MHEARD i beaconu

- Mapa jest dostępna także bez odebranych stacji i zawiera ręczne wysyłanie statusu UPRD.
- Usunięto osobną pozycję MHEARD z głównej nawigacji; tryby `Direct` i `UPRD` są dostępne bezpośrednio w prawym panelu MHEARD.
- Opcje klasycznego beaconu są widoczne w konfiguracji, a domyślny interwał wysyłania wynosi 30 minut.

## 2026-08-27 - UPRD i mapa topologii Packet Radio

- Dodano UPRD jako lekki system discovery nad AX.25 UI: osobny raport z locatora i listy MHEARD, walidacja payloadu oraz niezależna konfiguracja z interwałem i limitem stacji.
- UPRD korzysta z istniejącego MHEARD per TNC, przetwarza ramki w kolejności RX i zastępuje poprzedni raport reportera atomowo, bez tworzenia równoległej tabeli MHEARD.
- Rozszerzono widok MHEARD o tryb `Direct` i `UPRD`, a także dodano dane topologii i najlepszej znanej drogi dla aktualnego grafu.
- Mapa topologii działa jako osobny widok, filtruje rzeczywiste skonfigurowane TNC, zachowuje stabilne kolory interfejsów i aktualizuje graf dynamicznie bez przeładowania.
- Dwuklik na stację wykorzystuje istniejącą logikę Terminal/VIA do zestawienia połączenia po najlepszej znanej drodze.

## 2026-08-27 - czytelniejsze pole terminala

- Zakładki sesji pokazują tylko znak stacji, a pełny opis (znak, droga VIA i TNC) jest dostępny w dymku oraz nagłówku terminala; sesje przychodzące otrzymują te dane bezpośrednio z zestawionego łącza AX.25. W prawym górnym rogu wyświetlane są same nazwy TNC z dotychczasowymi diodami stanu.

- Nowo utworzona sesja pojawia się jako pierwsza od lewej, a starsze zakładki sesji przesuwają się kolejno w prawo.

- Dwuklik stacji w MHEARD nawiązuje połączenie z zapamiętaną drogą powrotną VIA; ścieżka digipeaterów jest automatycznie odwracana względem ostatnio odebranej ramki.

- Główny status terminala pokazuje wyłącznie stan sesji: Nawiązywanie połączenia, Połączono, Kończenie połączenia albo Rozłączono; wysyłanie ramek, oczekiwanie na ACK i odzyskiwanie T1 nie zastępują już stanu Połączono.

- Pole wpisywania ma wysokość dwóch wierszy, zawija długi tekst i przewija go pionowo, dzięki czemu można przejrzeć oraz poprawić całą wiadomość przed wysłaniem.
- Lista MHEARD pokazuje przy każdej stacji wyrównany do prawej czas w minutach od ostatniej odebranej ramki.
- Poprawiono jasny motyw dwuwierszowego pola terminala oraz układ MHEARD: czas jest czytelniejszy i pozostaje przy prawej krawędzi, a środkowa kolumna mieści opcjonalne `via`.
- Enter nadal wysyła wiadomość, a Shift+Enter pozwala ręcznie dodać podział wiersza. Wysłany tekst jest również zawijany w widoku rozmowy.
- Skrócono beacon: nie powtarza już własnego znaku ani QTH; pozostawia locator, oznaczenia `DIGI` i `UltimatePR` oraz listę ostatnio słyszanych stacji.
- Procedura XID używa teraz osobnych parametrów zarządzania TAPR `TM201` i `NM201`: jedna komenda oraz najwyżej dwie retransmisje co 10 sekund, niezależnie od ustawień `T1` i `N2`.
- Usunięto wyścig danych w teście wyczerpania T1/N2: test odczytuje teraz negocjowaną liczbę prób pod blokadą menedżera sesji.
- Długie wiadomości, również ciągi bez spacji, nie poszerzają już wierszy terminala; rozmowę można przewijać wyłącznie pionowo.
- Digipeater obsługuje połączenia `VIA` pomiędzy różnymi TNC: wybiera port docelowej stacji według najnowszego bezpośredniego wpisu MHEARD, a przy braku użytecznej trasy zachowuje retransmisję na porcie wejściowym.
- Usprawniono cross-TNC dla portów o różnych prędkościach: każdy TNC zachowuje niezależną kolejkę, retransmisje trybu connected nie są odrzucane jako duplikaty UI, a echo ramki z wykonanym VIA nie nadpisuje bezpośredniej trasy MHEARD.
- Gdy port stacji docelowej nie jest jeszcze znany, pierwsze ramki VIA są przekazywane do niezależnych kolejek wszystkich aktywnych TNC; odpowiedź uczy właściwej trasy i dalszy ruch trafia już tylko na wybrany port.
- Sprawdzanie i instalowanie aktualizacji nie korzysta już z limitowanego GitHub API: release publikuje `VERSION.txt`, a paczki i sumy SHA-256 są pobierane bezpośrednio z `main-latest` lub `dev-latest`.
- Status paczki terminala rozróżnia fizyczne nadanie od potwierdzenia AX.25: po wysłaniu pokazuje oczekiwanie na ACK, a końcowy zielony status `ACK` pojawia się dopiero po poprawnym `N(R)`.
- Przy wiadomości wieloramkowej potwierdzone części są na bieżąco scalane w jeden tekst; status poprzedniej paczki znika, a po zakończeniu pozostaje tylko końcowy `ACK` ostatniej paczki, np. `3/3`.

## 2026-08-26 - terminal AX.25 i bezpieczne TNC Proxy

- Uzupełniono pełną negocjację XID po zestawieniu łącza: N1 respektuje limit odbiorczy partnera, T1 i N2 wybierają większą wartość, a nieobsługiwane funkcje są obniżane do profilu modulo 8, implicit REJ i k=1.
- Brak odpowiedzi XID lub FRMR włącza parametry zgodności AX.25 v2.0; skorygowano także maskę funkcji implicit REJ w polu XID.
- Dodano osobną zakładkę `Terminal` w konfiguracji z prostymi opisami, widocznymi wartościami domyślnymi i przyciskiem przywracania całego profilu.
- Konfigurowalne są: zakończenie wiersza `CR` / `CRLF` / `LF`, czas odpowiedzi T1, kontrola bezczynności T3, liczba prób N2 oraz największa porcja danych N1/PACLEN.
- Domyślny profil pozostaje zgodny z używanym trybem AX.25 modulo 8: CR, T1=10 s, T3=300 s, N2=10 i N1=256 B.
- Odpowiedzi XID ogłaszają rzeczywiście skonfigurowane N1, T1 i N2; wartości są sprawdzane przed zapisaniem konfiguracji.
- Terminal zachowuje tekst UTF-8 bez automatycznej zamiany polskich znaków na ASCII.
- Naciśnięcie Enter wysyła również pusty wiersz, a transmisje trafiają do jednej kolejki i mogą zostać bezpiecznie anulowane podczas rozłączania.
- Historia zapisuje wiadomość wychodzącą dopiero po udanym wysłaniu.
- TNC Proxy rozdziela strumień na pełne ramki KISS. Dane wysłane przez klienta przekazuje do TNC i pozostałych klientów, bez echa do nadawcy; ramki odebrane z TNC rozsyła do wszystkich klientów.
- Proxy utrzymuje i ponawia połączenie TCP z TNC oraz blokuje komendy KISS `SET HARDWARE` i `RETURN`, które mogłyby zmienić stan wspólnego urządzenia.

## 2026-08-24 - korekta czasu odzyskiwania AX.25

- Ujednolicono domyślny czas `T1` do 10 sekund dla połączeń wychodzących i przychodzących oraz dla parametrów ogłaszanych przez `XID`. Wartość 3 sekund powodowała przedwczesne retransmisje przy pełnych ramkach `N1=256` i opóźnionej pracy stacji zdalnej.

## 2026-08-24 - poprawka kończenia sesji AX.25

- Po odebraniu zdalnego `DISC`, `DM` lub `SABM` oczekująca procedura odzyskiwania łącza natychmiast się kończy i nie wysyła ramek po zamknięciu sesji.

## 2026-08-24 - diagnostyczny eksport monitora

- Dodano wybór formatu eksportu monitora: dotychczasowy TXT czytelny dla człowieka oraz RAW w formacie JSONL do diagnostyki ramek AX.25.
- Eksport RAW zachowuje pełne wpisy monitora w kolejności chronologicznej, w tym zapis zakodowanych bajtów ramki.

## 2026-08-24 - zgodność i stabilność AX.25

- Ujednolicono obsługę protokołu AX.25 dla połączeń wychodzących terminala i połączeń przychodzących do usług; warstwa protokołu nie zależy od funkcji terminala, BBS ani NODE.
- Naprawiono routing ramek adresowanych do aktywnego połączenia wychodzącego: banner i pozostałe ramki `I` z LinBPQ trafiają teraz do właściwej sesji terminala i otrzymują `RR`, zamiast zostać przejęte przez obsługę połączeń przychodzących i odrzucone ramką `DM`.
- Opóźnione, poprawne `UA(F=1)` będące odpowiedziami na ponowione `SABM(P=1)` nie zrywają już świeżo zestawionego łącza; niezależne nieoczekiwane UA poza okresem zestawiania nadal uruchamiają obsługę błędu protokołu.
- Naprawiono kierunek ramek informacyjnych wysyłanych w sesjach przychodzących: ramki `I` są teraz prawidłowo oznaczane jako `command`, a nie `response`.
- Dodano stan `Timer Recovery` oraz pełne śledzenie zmiennych `V(S)`, `V(A)` i `V(R)` dla używanego profilu modulo 8.
- Uzupełniono procedurę stacji abonenckiej po wygaśnięciu T1: przed retransmisją ramki I wysyłane jest zapytanie `RR(P=1)`, a po N2 nieudanych próbach łącze przechodzi do stanu rozłączonego i czyści liczniki oraz stany RNR/REJ.
- Sesje przychodzące stosują tę samą procedurę odpytywania przed retransmisją oraz kończą lokalne połączenie przez `DISC(P=1)` z ponawianiem do otrzymania `UA(F=1)` lub `DM(F=1)`.
- Keepalive T3 sprawdza teraz odpowiedź zdalnej stacji i wykrywa utracone połączenie zamiast tylko wysyłać ramkę kontrolną.
- Zdalne `DM` i `DISC` natychmiast przerywają oczekującą transmisję, a równoczesne lub powtórzone `SABM` poprawnie zestawia albo resetuje łącze.
- Dodano przycisk „Wyczyść monitor” z potwierdzeniem, który usuwa wszystkie aktualnie buforowane ramki bez zatrzymywania dalszego monitorowania.
- Zestawienie i zakończenie połączenia rygorystycznie sprawdza `UA/DM` jako odpowiedź z bitem `F=1` na wysłane `SABM/DISC` z bitem `P=1`.
- Naprawiono nazwy typów ramek w monitorze po rozszerzeniu kodeka — `SABM`, `UA`, `RR`, `I` i pozostałe typy nie są już błędnie przesunięte ani pokazywane jako `?`.
- Ustawiono parametry domyślne AX.25 v2.2: T1 na 10 sekund, N2 na 10 prób, N1 na 256 bajtów oraz T3 na 5 minut.
- Poprawiono obsługę `RNR`: wysyłanie ramek informacyjnych jest wstrzymywane, gdy zdalna stacja zgłasza zajętość odbiornika, a gotowość jest sprawdzana ramkami `RR` z bitem `P`.
- Ramki informacyjne odebrane poza kolejnością uruchamiają teraz kontrolowaną procedurę `REJ` zamiast zwykłego potwierdzenia `RR`.
- Dodano obsługę `SREJ` dla obecnego, jednoklatkowego okna transmisyjnego.
- Zestawianie, przesyłanie danych i rozłączanie sprawdza normatywną semantykę bitów `P/F` oraz `C/R`; błędne kombinacje nie ustanawiają sesji.
- Sesje przychodzące odpowiadają na nadzorcze zapytania z bitem `P` oraz wysyłają `DM` do lokalnej usługi znajdującej się w stanie rozłączonym.
- Kodek rozpoznaje dodatkowe typy ramek AX.25 v2.2: `SABME`, `SREJ`, `FRMR`, `XID` i `TEST`.
- Dodano kodowanie i dekodowanie dwuoktetowego pola sterującego dla opcjonalnego modulo 128. Silnik sesji nadal negocjuje stabilny profil modulo 8, a nieobsługiwane zestawienie `SABME` odrzuca odpowiedzią `DM`.
- Dodano obsługę `XID`, która ogłasza rzeczywiste parametry profilu: half-duplex, implicit REJ, modulo 8, N1=256, okno k=1, T1=10 sekund i N2=10.
- Dodano odpowiedzi na ramki `TEST` z zachowaniem pola informacji oraz wymagane odpowiedzi na `UI(P=1)`.
- Zgodnie z AX.25 v2.2 błędy łącza są obsługiwane przez reset połączenia; aplikacja nie generuje wycofanych z tej wersji odpowiedzi `FRMR`.
- Enkoder odrzuca niedozwolone pole informacji w ramkach sterujących.
- Dodano testy regresyjne procedur `RNR`, `REJ`, `SREJ`, `P/F`, `C/R`, XID, TEST, modulo 128, nadzorczego odpytywania, odpowiedzi `DM`, kontrolowanego rozłączania, routingu sesji wychodzącej i opóźnionych odpowiedzi UA.

## 2026-08-24 - hostnames, niezależne sesje i czytelna historia

- Dodano usuwanie pojedynczej oglądanej rozmowy oraz całej historii rozmów w nowej zakładce „Baza danych”; każda operacja wymaga potwierdzenia.
- „Wyczyść terminal” usuwa wyłącznie zawartość bieżącego okna po potwierdzeniu i nie narusza zapisanej historii.

- Lista `web.allowed_addresses` przyjmuje teraz nazwy hostów oprócz adresów IP i sieci CIDR. Te same reguły chronią panel WWW oraz klientów TNC Proxy, a nazwy są ponownie rozwiązywane przez DNS przy sprawdzaniu połączenia.
- Poprawiono dostęp przez hostname dla klientów używających strefowych adresów IPv6 link-local, np. `fe80::…%eth0`; reguła `::` obejmuje je teraz prawidłowo w WWW i TNC Proxy.
- Każda zakładka terminala ma własny stan połączenia i osobny przycisk `×` służący do trwałego zamknięcia sesji.
- Przycisk `Rozłącz sesję` kończy tylko bieżące połączenie, zachowując zakładkę, treść terminala i możliwość ręcznego ponownego połączenia.
- Usunięto automatyczne ponawianie połączeń i jego przełącznik z głównego paska. Pola nowego połączenia wykorzystują całą zwolnioną szerokość.
- Podgląd historii i beaconów jest oddzielony od aktywnych sesji, dzięki czemu nie pokazuje ani nie przejmuje stanu innego połączenia.
- Tytuł przeglądanej historii jest wyraźny i wyśrodkowany w górnym pasku terminala.
- Wiadomości w historii pokazują datę i godzinę całej wiadomości, a nie osobnych pakietów transmisji.
- Początek i koniec każdego połączenia są trwale zapisywane i przedstawiane jako graficzne separatory: zielony `Połączono` oraz czerwony `Rozłączono`, oba z datą i godziną.
- Ręczne rozłączenie i zamknięcie aktywnej zakładki wysyła najpierw skonfigurowane pożegnanie, czeka na zakończenie transmisji, a dopiero później wysyła ramkę AX.25 `DISC`.
- Ujednolicono czytelność całego interfejsu: zwiększono najmniejsze opisy, metadane, statusy i podpowiedzi, wzmocniono ich krój oraz kontrast w jasnym i ciemnym motywie.
- Przed zapisaniem zmienionego adresu panelu WWW aplikacja sprawdza, czy może go otworzyć. Brak uprawnień lub zajęty port nie nadpisze już działającej konfiguracji ani nie odetnie panelu po restarcie.

## 2026-08-21 - współdzielenie TNC przez aplikacje zewnętrzne

- Dodano opcjonalny, wbudowany proxy KISS TCP dla każdego portu TNC.
- Przy wyłączonym proxy UltimatePR łączy się bezpośrednio z TNC, np. na porcie `8001`.
- Przy włączonym proxy UltimatePR i zewnętrzne aplikacje KISS mogą korzystać wspólnie z portu klientów, domyślnie `127.0.0.1:8101`.
- Dodano konfigurację `tncproxy_enabled` oraz `tncproxy_port` w zakładce TNC i w YAML.
- Lista `web.allowed_addresses` ogranicza również klientów TNC Proxy.
- Proxy przekazuje ramki między TNC a wszystkimi podłączonymi klientami.

## 2026-08-21 - bezpieczne polecenia zdalne

- Polecenia zdalne wymagają teraz prefiksu `/` i osobnej linii: `/I` oraz `/MH`.
- Dodano pomoc dostępną przez `/H` i `/?`.
- Domyślne powitanie informuje korespondenta o dostępnych poleceniach; własne powitania nie są nadpisywane.
- Długie wiadomości są dzielone na ramki w miejscu ostatniej spacji lub końca linii przed limitem `paclen`, bez przecinania zwykłych słów.
- Wychodzący tekst i status transmisji są wyrównane do prawej; każda paczka wieloramkowej wiadomości pozostaje widoczna w osobnym wierszu z własnym statusem.

## 2026-08-21 - stabilność sesji i eksport monitora

- Stare lub niepasujące potwierdzenia `RR` nie powodują już natychmiastowych, wielokrotnych retransmisji tej samej wiadomości.
- `REJ` nadal uruchamia kontrolowane ponowienie właściwej ramki zgodnie z numerem sekwencji.
- Monitor ramek można wyeksportować do czytelnego pliku TXT w kolejności chronologicznej.

## 2026-08-21 - makra terminala

- Makra `{CALL}`, `{NAME}`, `{LOC}`, `{QTH}` i `{REMOTE}` są rozwijane również w wiadomościach wpisywanych podczas rozmowy, bezpośrednio przed wysłaniem.
- Dane makr są pobierane z aktualnej konfiguracji w sekcji `Stacja operatora`.
- Pożegnanie wyświetlane po zdalnym rozłączeniu pokazuje już podstawione wartości zamiast surowych nazw makr.

## 2026-08-21 - połączenia przychodzące przez DIGI

- Sesje przychodzące zapamiętują trasę DIGI odebraną w ramce `SABM`.
- Ramki zwrotne `UA`, `RR`, `I` i `DISC` są wysyłane odwróconą trasą, dzięki czemu korespondent może je odebrać i potwierdzić.
- Usunięto przyczynę ponawiania powitania co kilka sekund i późniejszego zapętlenia `DISC` / `SABM` przy łączności przez digipeater.
- Polskie znaki wpisane w polu rozmowy są przed transmisją zamieniane na czytelne odpowiedniki ASCII, np. `ąśćę` na `asce`.

## 2026-08-20

- Ikony w górnym pasku zamiast napisów dla motywu, dźwięku, informacji i konfiguracji.
- Widok `Info` z prostym manualem oraz podstawowymi założeniami aplikacji.
- `Changelog` jako podzakładka w `Info`.
- Komendy `MH` oraz `I` / `INFO` z automatycznymi odpowiedziami w terminalu.
- Edytowalne wiadomości terminala: powitanie, pożegnanie i info.

## 2026-08-20 - uzupełnienie

- Jaśniejsze i czytelniejsze karty w widoku `Info`.
- Jasne pola edycji wiadomości terminala: powitanie, pożegnanie i info.
- Lepszy kontrast tekstu changelogu.

## 2026-08-20 - info cleanup

- Usunięty panel stanu aplikacji z `Info`.
- Usunięte `uptime` z górnego paska i z `Info`.
- Dodany link do repozytorium GitHub na dole widoku `Info`.
