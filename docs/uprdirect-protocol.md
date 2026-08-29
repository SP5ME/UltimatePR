# Protokół UPRdirect

## Zakres

UPRdirect (UPRD) jest krótkim raportem lokalnej wiedzy RF przesyłanym jako
standardowa ramka AX.25 UI. Ramka nie używa digipeaterów w polu VIA.

- Destination: `UPR`
- Source: znak stacji nadającej, z istniejącym SSID AX.25
- Type: `UI`
- PID: `0xF0`
- Information: payload opisany poniżej

## Payload

Payload ma postać:

```text
<EXISTING_ENCODED_CALL>|<STATUS_BYTE>|<LOCATOR>|<HEARD_LIST>
```

Pierwsze pole jest istniejącym zakodowanym znakiem. Jego algorytm, długość,
powiązanie ze znakiem AX.25 i walidacja pozostają bez zmian. Przesunięcie jest
obliczane z locatora i listy heard tak jak dotychczas.

Separator `|` ma wartość ASCII `0x7C`. `STATUS_BYTE` jest dokładnie jednym
bajtem binarnym, a nie tekstem. Payload należy przenosić i analizować z jawną
długością, ponieważ `0x00` nie kończy informacji.

Locator jest opcjonalnym sześci znakowym lokatorem Maidenhead. Lista heard jest
tekstem ASCII z unikalnymi znakami oddzielonymi przecinkami; jej kolejność jest
kolejnością raportu, a limit wynika z konfiguracji UPRD.

## Status operatora

| Bit | Nazwa | Znaczenie |
| --- | --- | --- |
| 0 | `OPERATOR_ABSENT` | `0`: operator obecny, `1`: operator nieobecny |
| 1-7 | `RESERVED` | Nadajnik ustawia `0`; odbiornik ignoruje |

Obecne wartości to `0x00` dla operatora obecnego i `0x01` dla operatora
nieobecnego. Odbiornik interpretuje wyłącznie `status & 0x01`, więc na przykład
`0x05` oznacza operatora nieobecnego, a ustawione bity zarezerwowane nie są
błędem.

Źródłem statusu nadajnika jest istniejący stan aktywnego panelu operatora.
Nie tworzy się osobnego mechanizmu wykrywania obecności.

## Przykłady payloadu

Dla zakodowanego znaku `1YEVN`, locatora `KO02MD` i listy
`SR5DDD,SQ9MDD`, payload operatora obecnego ma bajty:

```text
31 59 45 56 4E 7C 00 7C 4B 4F 30 32 4D 44 7C 53 52 35 44 44 44 2C 53 51 39 4D 44 44
```

Operator nieobecny różni się wyłącznie bajtem statusu:

```text
31 59 45 56 4E 7C 01 7C 4B 4F 30 32 4D 44 7C 53 52 35 44 44 44 2C 53 51 39 4D 44 44
```

W obu przypadkach końcowe pola i kolejność listy są identyczne. Powyższe
przykłady pokazują Information payload; pełna ramka zawiera przed nim adresy,
kontrolkę UI i PID zgodnie ze standardem AX.25.

## Odbiór i monitor

Odbiorca najpierw sprawdza istniejący zakodowany znak. Dopiero po pozytywnej
walidacji odczytuje jeden bajt statusu, następnie locator i listę heard.
Informacja o obecności operatora jest zapisywana przy raporcie odebranej stacji
jako `operator_present`.

Monitor zachowuje pełną ramkę RAW/HEX, w tym rzeczywisty bajt `00` albo `01`.
Dla ramek Destination `UPR` pokazuje dodatkowo status w formie czytelnej dla
człowieka, bez zamiany statusu na ASCII w transmisji.
