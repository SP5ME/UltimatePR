# KISS i kodowanie terminala Packet Radio

UltimatePR używa KISS wyłącznie jako transportu surowych ramek dla Packet
Radio. Funkcje APRS nie należą do zakresu projektu.

## Zgodność ramek KISS

Kodek realizuje format opisany przez K3MC i KA9Q:

- każda ramka rozpoczyna się i kończy bajtem `FEND` (`C0`);
- `FEND` w danych jest przesyłany jako `FESC TFEND` (`DB DC`);
- `FESC` w danych jest przesyłany jako `FESC TFESC` (`DB DD`);
- kolejne bajty `FEND` nie tworzą pustych ramek;
- dane odebrane przed pierwszym `FEND` są odrzucane jako zakłócenie;
- błędna sekwencja escape jest raportowana, ale dekoder odzyskuje
  synchronizację bez zatrzymywania portu;
- górne cztery bity bajtu typu określają port KISS 0–15, a dolne cztery
  bity określają komendę.

Ramki danych mają komendę `0`. Ramki innych komend odebrane od TNC są
ignorowane zgodnie ze specyfikacją.

## Port i parametry TNC

Każdy port KISS TCP może opcjonalnie określić:

- `kiss_port` — port urządzenia od 0 do 15, domyślnie 0;
- `kiss_txdelay` — opóźnienie PTT w jednostkach 10 ms;
- `kiss_persistence` — parametr P od 0 do 255;
- `kiss_slottime` — czas szczeliny w jednostkach 10 ms;
- `kiss_txtail` — czas TX tail w jednostkach 10 ms;
- `kiss_full_duplex` — `true` albo `false`.

Parametry pozostawione puste nie są wysyłane. Dzięki temu domyślna
konfiguracja nie nadpisuje ustawień Dire Wolfa ani fizycznego TNC. Ustawione
komendy są wysyłane po każdym ponownym zestawieniu połączenia z TNC.

## Kodowanie tekstu

Kodowanie jest właściwością sesji terminala, a nie ramki KISS. KISS i AX.25
przenoszą bajty, dlatego konwersja następuje dopiero pomiędzy sesją radiową a
interfejsem WWW.

Dostępne tryby:

- `auto` — poprawny UTF-8 pozostaje UTF-8, inne bajty są dekodowane jako CP437;
- `utf-8`;
- `cp437` — grafika i tekst starych systemów DOS/BBS;
- `cp850`;
- `windows-1250`;
- `iso-8859-2`.

Wybrane kodowanie jest zapisywane razem z kartą sesji. Dotyczy zarówno
odbioru, jak i nadawania oraz tekstu zapisywanego w historii. Gdy automatyczne
rozpoznanie nie odpowiada konkretnemu BBS-owi, należy wybrać kodowanie jawnie
przed połączeniem.

Źródło specyfikacji: <https://www.ax25.net/kiss.aspx>.
