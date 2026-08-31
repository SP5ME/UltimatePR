# Model noda i BBS

## Rozdzielenie odpowiedzialności

Node jest powłoką usługową AX.25. Zarządza portami (KISS/AXUDP), bezpośrednimi
sąsiadami, trasami oraz lokalnymi usługami. BBS jest jedną z usług noda i
zarządza użytkownikami, pocztą, MID/BID, adresami hierarchicznymi oraz
forwardingiem. Pełna warstwa sieciowa NET/ROM pozostaje osobnym etapem.

Łącze AXUDP nie jest forwardingiem poczty. Tworzy jedynie drogę dla ramek:

```text
AXUDP -> AX.25/Node -> sesja do BBS -> forwarding poczty
```

## Porty i sąsiedzi

Każdy port ma własny identyfikator. Sąsiad noda wskazuje port i dokładny znak
AX.25. Statyczna trasa wskazuje sąsiada przez jego identyfikator, a nie adres
IP. Pozwala to zmienić transport bez zmiany tabeli routingu.

AXUDP obsługuje opcjonalny FCS i listę dozwolonych adresów IP/CIDR. Wartość FCS
musi być uzgodniona z drugą stroną. Port 10093 jest wartością przykładową;
operatorzy muszą uzgodnić rzeczywisty port i oba kierunki połączenia.

## Poczta i forwarding

Prywatny adres ma postać `CALL@BBS.[#AREA.][REGION.]COUNTRY.CONTINENT`, np.
`SP5ME@SP5AAA.#PL.POL.EURO`. Biuletyn przechowuje oddzielnie kategorię i
designator dystrybucji, np. `ALL@POL`.

Każda lokalna instancja wiadomości ma MID. Biuletyn ma ponadto niezmienny BID,
który służy do blokowania duplikatów. Prywatna wiadomość do konkretnej stacji
nie otrzymuje automatycznego BID. Partner forwardingu posiada:

- transport `node` albo bezpośredni `telnet`;
- znak BBS i opcjonalny węzeł pośredni;
- harmonogram;
- trasy prywatne i obszary biuletynów;
- limity sesji i wielkości danych.

Planner wybiera wiadomości do kolejki partnera. Warstwa przesyłania implementuje
klasyczny protokół TAPR BBS: SID z cechami `H$`, `SP`/`SB`, `OK`/`NO`, nagłówki
`R:`, zakończenie Ctrl-Z oraz zmianę kierunku `F>`. Forwarding wymaga pełnego
lokalnego adresu hierarchicznego TAPR.

## Stan implementacji

- gotowe: konfiguracja noda/BBS, statyczne rozwiązywanie tras, AXUDP z FCS i
  allowlistą, osobne MID/BID, parser x.3.4, designatory dystrybucji, wybór
  wiadomości oraz klasyczny forwarding TAPR;
- następne: uwierzytelnianie partnerów, wielosesyjny node przychodzący,
  dynamiczne NET/ROM i panel konfiguracji. CONNECT przez statyczną trasę
  zestawia teraz rzeczywiste łącze AX.25 do sąsiada i przekazuje sesję do
  kolejnego noda.
- nieaktywne domyślnie: przykładowe łącze i partner SR5DDD. Wymagają prawdziwych
  parametrów uzgodnionych z jego operatorem.
