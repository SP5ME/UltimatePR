# Changelog

## 2026-08-24 - hostnames, niezależne sesje i czytelna historia

- Lista `web.allowed_addresses` przyjmuje teraz nazwy hostów oprócz adresów IP i sieci CIDR. Te same reguły chronią panel WWW oraz klientów TNC Proxy, a nazwy są ponownie rozwiązywane przez DNS przy sprawdzaniu połączenia.
- Każda zakładka terminala ma własny stan połączenia i osobny przycisk `×` służący do trwałego zamknięcia sesji.
- Przycisk `Rozłącz sesję` kończy tylko bieżące połączenie, zachowując zakładkę, treść terminala i możliwość ręcznego ponownego połączenia.
- Usunięto automatyczne ponawianie połączeń i jego przełącznik z głównego paska. Pola nowego połączenia wykorzystują całą zwolnioną szerokość.
- Podgląd historii i beaconów jest oddzielony od aktywnych sesji, dzięki czemu nie pokazuje ani nie przejmuje stanu innego połączenia.
- Tytuł przeglądanej historii jest wyraźny i wyśrodkowany w górnym pasku terminala.
- Wiadomości w historii pokazują datę i godzinę całej wiadomości, a nie osobnych pakietów transmisji.
- Początek i koniec każdego połączenia są trwale zapisywane i przedstawiane jako graficzne separatory: zielony `Połączono` oraz czerwony `Rozłączono`, oba z datą i godziną.

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
- Wychodzący tekst i status transmisji są wyrównane do prawej; po zakończeniu wieloramkowej wiadomości pozostaje tylko końcowy status ostatniej paczki.

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
