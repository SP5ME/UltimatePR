# Konfiguracja NODE

Panel `Konfiguracja -> NODE` opisuje ustawienia używane przez aktualny runtime
UltimatePR. NODE korzysta z tych samych logicznych interfejsow co terminal i
UPRdirect. Pole `Port` w starym YAML jest prezentowane jako `Interface`.

## Ustawienia

| Opcja | Znaczenie | Zakres / jednostka | Domyslnie | Wplyw na runtime |
| --- | --- | --- | --- | --- |
| `node.enabled` | Uruchamia usluge NODE AX.25 i lokalny listener TCP | wl./wyl. | `false` w trybie station | Dziala |
| `server.callsign` + `server.ssid` | Adres AX.25 noda | callsign 1-6 znakow, SSID 0-15 | znak stacji, SSID 7 | Dziala |
| `node.alias` | Krotka nazwa noda i prompt powloki | 1-6 znakow | pierwsze 6 znakow callsign | Dziala |
| `node.listen` | Lokalny listener dla klientow TCP | `host:port` | `127.0.0.1:8010` | Dziala; nie jest radiowym interfejsem |
| `node.language` | Jezyk powloki NODE | `pl` / `en` | jezyk aplikacji | Dziala |
| `node.neighbors[].id` | Stabilny identyfikator sasiada | niepusty, unikalny | brak | Dziala |
| `node.neighbors[].callsign` | Callsign bezposrednio osiagalnego noda | adres AX.25 | brak | Dziala |
| `node.neighbors[].port` | Interface UltimatePR uzywany do sasiada | aktywny ID portu lub kanalu | brak | Dziala |
| `node.neighbors[].quality` | Preferencja bezposredniego polaczenia | 0-255 | brak | Dziala; nie jest RSSI |
| `node.routes[].destination` | Cel trasy statycznej | nazwa celu | brak | Dziala |
| `node.routes[].via` | ID sasiada, przez ktorego idzie trasa | aktywny neighbor | brak | Dziala |
| `node.routes[].quality` | Preferencja trasy przy wielu drogach | 0-255 | brak | Dziala; wyzsza wartosc jest sortowana pierwsza |
| `node.netrom_enabled` | Wymiana i uczenie tras NET/ROM | wl./wyl. | `false` | Dziala |
| `node.netrom_mnemonic` | Mnemonic nadawany w broadcastach NODES | do 6 znakow | alias noda, gdy puste | Dziala |
| `node.netrom_interval_seconds` | Odstep broadcastow NODES | 1-86400 s | runtime: 3600 s | Dziala po wlaczeniu NET/ROM |
| `node.netrom_obsolescence` | Poczatkowa liczba cykli zycia trasy dynamicznej | 1-255 cykli | runtime: 6 | Dziala; statyczne trasy nie wygasaja |
| `node.netrom_min_quality` | Minimalna jakosc przyjmowanego broadcastu | 1-255 | runtime: 1 | Dziala; nie jest pomiarem sygnalu |
| `node.netrom_max_destinations` | Limit destinations w generowanym broadcastcie | 1-255 wpisow | brak, runtime ogranicza liste | Dziala |
| `node.welcome_message` / `goodbye_message` | Wiadomosci sesji NODE | tekst | komunikat wbudowany | Dziala; makra `{CALL}` i `{REMOTE}` |
| `node.neighbors[].locked` | Zachowane pole zgodnosci starego YAML | wl./wyl. | zalezne od starego configu | Obecnie nie wplywa na router; nie jest pokazywane w panelu |

`Quality` jest wartoscia kosztu/preferencji trasy. Przykladowo trasa przez
`WAW02` z Quality `220` jest preferowana przed trasa przez `WAW03` z Quality
`180`. Nie nalezy interpretowac tej liczby jako RSSI, SNR ani sily sygnalu.

`Obsolescence` dotyczy tras nauczonych z broadcastow NET/ROM. Router zmniejsza
ten licznik przy starzeniu i usuwa wpis po wygasnieciu. Trasy skonfigurowane
statycznie pozostaja w tabeli.

## Przyklad prostego noda VHF

```text
Enabled: Yes
Callsign: SR5XYZ-5
Alias: WAW01
Interface: VHF-1200

Neighbor: SR5ABC-5
Interface: VHF-1200
Quality: 200

Static route:
Destination: KRK01
Via: SR5ABC-5
Quality: 180
```

`SR5ABC-5` jest bezposrednim sasiadem przez interface `VHF-1200`. Ruch do
`KRK01` jest kierowany przez tego sasiada.

## Multi-interface

Przyklad:

```text
WAW02 -> VHF-1200 -> Quality 180
WAW03 -> UHF-9600 -> Quality 220
```

NODE moze odebrac sesje przez jeden interface i zestawic dalsze polaczenie
przez drugi. Kilka tras do tego samego celu jest sortowanych wedlug Quality;
aktywny failover po bledzie polaczenia nie jest jeszcze osobnym mechanizmem
konfiguracyjnym runtime.

## Zrodla i status

- [TAPR FTP Archive](https://tapr.org/ftp-archive/)
- [TAPR Packet Status, NET/ROM routing](https://files.tapr.org/psr/psr028.pdf)
- [Kod routera](../internal/node/router.go)
- [Uruchomienie NODE](../cmd/server/main.go)

Interfejsy sa wybierane z aktywnych wpisow `ports[].id` oraz logicznych kanalow
`ports[].channels`. Nieistniejacy lub wylaczony interface oraz nieistniejacy
sasiad nie przechodza walidacji konfiguracji.
