# Wersjonowanie aplikacji

UltimatePR używa automatycznego numeru buildu zgodnego z SemVer:

- `MAJOR.MINOR.PATCH` dla wydań stabilnych, na przykład `0.4.4`;
- `MAJOR.MINOR.BUILD-KANAŁ+SHA` dla każdego pushu do `dev` lub `main`, na przykład `0.4.237-main+abc1234`;
- lokalny build bez procesu wydawniczego ma przyrostek `-local`.

Numer `BUILD` jest pełną liczbą commitów w historii repozytorium. Każdy nowy commit wypchnięty do obsługiwanej gałęzi zwiększa więc ostatni liczbowy człon wersji. `KANAŁ` wskazuje `main` albo `dev`, a `SHA` jest skrótem commita użytego do zbudowania pakietu.

Workflow pobiera pełną historię Git (`fetch-depth: 0`), aby licznik nie resetował się do `1` w GitHub Actions. Ponowne uruchomienie workflow dla tego samego commita zachowuje ten sam numer; push zawierający kilka commitów zwiększa numer o ich liczbę.

Wydanie stabilne powstaje z taga `vMAJOR.MINOR.PATCH`; numer taga jest jednocześnie wersją aplikacji. Nie dopisuje się do niego daty.

## Changelog

Nowe zmiany wpisujemy na początku `internal/web/static/changelog.md` oraz `internal/web/static/changelog.en.md` w sekcji `Unreleased`, grupując je pod nagłówkami `Added`, `Changed`, `Fixed` i `Removed`. Po wydaniu sekcję przenosi się pod numer konkretnej wersji, na przykład `0.4.4`.

Starsze wpisy datowane pozostają bez zmiany, ponieważ nie mają wiarygodnie przypisanych numerów wersji.
