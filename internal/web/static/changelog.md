# Changelog

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
