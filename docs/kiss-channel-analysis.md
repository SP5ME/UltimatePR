# Kanały KISS — analiza i temat na przyszłość

## Stan obecny

UltimatePR korzysta z KISS TCP i ma obecnie stałą, standardową konfigurację
jednokanałowego TNC:

- połączenie z TNC jest konfigurowane przez adres IP i port TCP;
- transmisja używa portu KISS `0` i komendy DATA `0`;
- pierwszy bajt ramki KISS DATA ma zatem wartość `0x00`;
- odbiór akceptuje ramki DATA wyłącznie z portu KISS `0`;
- TX i RX używają tej samej numeracji;
- w konfiguracji i GUI nie ma osobnego parametru kanału KISS.

Generowana ramka ma postać:

```text
FEND  0x00  AX.25 DATA...  FEND
```

Takie zachowanie jest zgodne ze standardem KISS dla pierwszego portu i działa z
Dire Wolf oraz typowym jednokanałowym TNC. Nie należy dodawać wyjątku zależnego
od Dire Wolf.

## Ważne rozróżnienie

Widoczny w GUI numer, na przykład `8001`, jest portem sieciowym TCP, a nie
numerem portu zapisanym w górnych czterech bitach bajtu komendy KISS.

W KISS:

```text
bity 7..4 = numer portu KISS (0..15)
bity 3..0 = komenda (0 = DATA)
```

Przykłady:

```text
port KISS 0 + DATA = 0x00
port KISS 1 + DATA = 0x10
```

## Dlaczego nie zmieniamy tego teraz

Wersja ze stałym portem KISS `0` działa poprawnie w obecnych instalacjach.
Dodanie pola kanału bez potwierdzonego urządzenia wielokanałowego zwiększa
ryzyko przypadkowego ustawienia portu `1`, który w standardzie oznacza drugi,
a nie pierwszy port TNC.

Wpis `channel:` pojawiający się w starszym przykładzie testowym nie jest częścią
modelu konfiguracji i nie wpływa na sterownik KISS.

## Możliwe rozszerzenie w przyszłości

Jeżeli pojawi się potwierdzona potrzeba obsługi wielokanałowego TNC, należy
dodać jawne pole:

```yaml
kiss_port: 0
```

Założenia implementacji:

1. Zakres wartości `0..15`.
2. Domyślna wartość `0`, również dla istniejących konfiguracji.
3. Brak mapowania `1 -> 0`; wartość ma oznaczać rzeczywisty port KISS.
4. Ten sam port musi być stosowany przez TX i filtr RX.
5. GUI powinno rozdzielać `Port TCP` od `Port KISS` i wyjaśniać, że dla TNC
   jednokanałowego używa się `0`.
6. Nie dodawać profilu „Dire Wolf compatibility”, ponieważ Dire Wolf używa
   standardowej numeracji.
7. Tryb kompatybilności producenta dodawać tylko po przechwyceniu ramek, które
   potwierdzą niestandardowe zachowanie konkretnego urządzenia.

## Testy wymagane przed wdrożeniem rozszerzenia

- skonfigurowany port `0` generuje bajt komendy DATA `0x00`;
- skonfigurowany port `1` generuje bajt komendy DATA `0x10`;
- dekoder poprawnie wyodrębnia port z górnych czterech bitów;
- filtr RX przyjmuje wyłącznie skonfigurowany port;
- brak `kiss_port` w starej konfiguracji zachowuje port `0`;
- standardowa ramka dla Dire Wolf ma postać `C0 00 ... C0`.

Przed zmianą warto zebrać zrzuty rzeczywistych ramek TX/RX z urządzenia, które
nie działa na porcie KISS `0`. Dopiero taki materiał uzasadnia rozszerzenie lub
izolowaną warstwę kompatybilności.
