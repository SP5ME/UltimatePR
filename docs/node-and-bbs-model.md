# Model noda i BBS

## Rozdzielenie odpowiedzialności

Node jest routerem AX.25. Zarządza portami (KISS/AXUDP), bezpośrednimi
sąsiadami, trasami oraz lokalnymi usługami. BBS jest jedną z usług noda i
zarządza użytkownikami, pocztą, BID, adresami hierarchicznymi oraz forwardingiem.

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

Prywatny adres ma postać `CALL@BBS.HA`. Biuletyn używa obszaru, np. `POL` lub
`EU`. Każda nowa wiadomość otrzymuje unikalny BID. Partner forwardingu posiada:

- transport `node` albo bezpośredni `telnet`;
- znak BBS i opcjonalny węzeł pośredni;
- harmonogram;
- trasy prywatne i obszary biuletynów;
- limity sesji i wielkości danych.

Planner wybiera wiadomości do kolejki partnera. Warstwa protokołu przesyłania
pozostaje osobna. Obecnie używa nieskompresowanego FBB; kompresję B1F/B2F można
dodać bez zmiany routingu i magazynu wiadomości.

## Stan implementacji

- gotowe: konfiguracja noda/BBS, statyczne rozwiązywanie tras, AXUDP z FCS i
  allowlistą, BID, adres `CALL@BBS`, wybór wiadomości dla partnerów;
- następne: wielosesyjny node przychodzący, NET/ROM, trwałe stany kolejek,
  protokół forwardingowy i panel konfiguracji;
- nieaktywne domyślnie: przykładowe łącze i partner SR5DDD. Wymagają prawdziwych
  parametrów uzgodnionych z jego operatorem.
