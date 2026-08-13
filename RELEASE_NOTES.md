# UltimatePR v0.4.3

## Co nowego

- panel WWW domyślnie dostępny w sieci na `0.0.0.0:8080`;
- prosty panel logowania z jednym kontem administratora;
- domyślne dane pierwszego logowania: `admin` / `packet`;
- nowa zakładka **Konfiguracja → Aplikacja**;
- zmiana hasła z obowiązkowym powtórzeniem nowego hasła;
- hasła są zapisywane jako PBKDF2-SHA256, a nie jako jawny tekst;
- ograniczanie dostępu do panelu według adresów IP i sieci CIDR;
- przycisk wylogowania;
- poprawiona dokumentacja instalacji i bezpieczeństwa.

Po pierwszym zalogowaniu należy zmienić domyślne hasło.

## Wybór paczki

Na serwerze wykonaj:

```sh
uname -m
```

- `x86_64` → `ultimatepr-v0.4.3-linux-amd64.tar.gz`
- `aarch64` → `ultimatepr-v0.4.3-linux-arm64.tar.gz`
- `armv7l` → `ultimatepr-v0.4.3-linux-armv7.tar.gz`

## Alpine Linux — instalacja przez SSH

Poniższy przykład jest przeznaczony dla `x86_64`. Dla ARM zmień nazwę paczki
i katalogu zgodnie z tabelą powyżej.

```sh
cd /root
wget https://github.com/SP5ME/UltimatePR/releases/download/v0.4.3/ultimatepr-v0.4.3-linux-amd64.tar.gz
tar -xzf ultimatepr-v0.4.3-linux-amd64.tar.gz
cd ultimatepr-v0.4.3-linux-amd64
chmod +x ultimatepr
./ultimatepr -config config.example.yaml
```

W logu powinien pojawić się adres `http://0.0.0.0:8080`. Sprawdź adres serwera:

```sh
ip -4 addr
```

Na komputerze w tej samej sieci otwórz:

```text
http://ADRES_IP_SERWERA:8080
```

Pierwsze logowanie:

```text
Użytkownik: admin
Hasło: packet
```

Ostrzeżenie o braku połączenia z `127.0.0.1:8001` oznacza, że Direwolf lub
inny TNC KISS nie jest jeszcze uruchomiony. Nie blokuje ono panelu WWW.

## Debian, Ubuntu i Raspberry Pi OS

Zainstaluj wymagane narzędzia:

```sh
sudo apt update
sudo apt install -y ca-certificates wget tar
```

Następnie pobierz paczkę właściwą dla wyniku `uname -m`, rozpakuj i uruchom ją
tak samo jak w instrukcji dla Alpine. Przykład AMD64:

```sh
wget https://github.com/SP5ME/UltimatePR/releases/download/v0.4.3/ultimatepr-v0.4.3-linux-amd64.tar.gz
tar -xzf ultimatepr-v0.4.3-linux-amd64.tar.gz
cd ultimatepr-v0.4.3-linux-amd64
chmod +x ultimatepr
./ultimatepr -config config.example.yaml
```

## Pozostałe dystrybucje Linux

Binarka jest kompilowana z `CGO_ENABLED=0`, dlatego nie wymaga glibc i powinna
działać jako samodzielny plik na typowych dystrybucjach Linux. Potrzebne są
jedynie `tar` do rozpakowania oraz narzędzie do pobrania pliku, np. `wget` lub
`curl`.

## Aktualizacja z v0.4.2

Zatrzymaj starszy proces przez `Ctrl+C`, pobierz i rozpakuj nową paczkę. Możesz
zachować dotychczasowy plik konfiguracji, ale dodaj do sekcji `web`:

```yaml
web:
    listen: 0.0.0.0:8080
    username: admin
    password_hash: ""
    allowed_addresses:
        - 0.0.0.0
```

Puste `password_hash` oznacza domyślne hasło `packet`. Po zalogowaniu ustaw nowe
hasło w zakładce **Konfiguracja → Aplikacja**.

## Kontrola pobranego pliku

Po pobraniu możesz sprawdzić sumę SHA-256:

```sh
sha256sum -c SHA256SUMS.txt
```

## Windows

Od wersji v0.4.2 workflow wydania publikuje wyłącznie paczki Linux. Projekt
można nadal uruchamiać lub kompilować lokalnie na Windows ze źródeł przy użyciu
Go, ale gotowa binarka Windows nie jest częścią tego wydania.
