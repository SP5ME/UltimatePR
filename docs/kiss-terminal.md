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

## Współdzielenie TNC przez TNC Proxy

Dla portu typu `kiss-tcp` można włączyć wbudowany proxy KISS:

```yaml
ports:
  - id: radio-2m
    type: kiss-tcp
    host: 127.0.0.1
    port: 8001
    tncproxy_enabled: true
    tncproxy_listen: 127.0.0.1:8101
```

W tym trybie UltimatePR łączy się z proxy na porcie `8101`, a proxy łączy się
z właściwym TNC na porcie `8001`. Na `8101` mogą równocześnie łączyć się
UltimatePR i zewnętrzne aplikacje KISS TCP. Gdy `tncproxy_enabled` jest
wyłączone, UltimatePR łączy się bezpośrednio z `host:port`.

Port `tncproxy_listen` musi być unikalny dla każdego włączonego portu TNC.
Jeśli aplikacja zewnętrzna działa na innym komputerze, użyj np.
`0.0.0.0:8101` i ogranicz dostęp regułami zapory sieciowej.

## Kodowanie tekstu

Kodowanie jest na sztywno ustawione na `UTF-8`. KISS i AX.25 przenoszą bajty,
dopiero warstwa terminala interpretuje je jako tekst, ale w tej aplikacji nie ma
już wyboru innego kodowania.

To oznacza, że:

- interfejs WWW zawsze pokazuje i wysyła tekst jako UTF-8;
- zapis sesji nie przechowuje alternatywnego kodowania;
- historia i monitor korzystają z tego samego założenia.

Źródło specyfikacji: <https://www.ax25.net/kiss.aspx>.
