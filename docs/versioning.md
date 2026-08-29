# Wersjonowanie aplikacji

UltimatePR używa wersjonowania SemVer:

- `MAJOR.MINOR.PATCH` dla wydań stabilnych, na przykład `0.4.4`;
- `MAJOR.MINOR.PATCH-dev.N+SHA` dla pakietów z gałęzi `dev` lub `main`, na przykład `0.4.4-dev.123+abc1234`;
- lokalny build bez procesu wydawniczego ma przyrostek `+local`.

Numer `N` jest liczbą commitów w historii sprawdzanego repozytorium, a `SHA` jest skrótem commita użytego do zbudowania pakietu. Dzięki temu można odróżnić zarówno generację aplikacji, jak i konkretny build.

Wydanie stabilne powstaje z taga `vMAJOR.MINOR.PATCH`; numer taga jest jednocześnie wersją aplikacji. Nie dopisuje się do niego daty.

## Changelog

Nowe zmiany wpisujemy na początku `internal/web/static/changelog.md` oraz `internal/web/static/changelog.en.md` w sekcji `Unreleased`, grupując je pod nagłówkami `Added`, `Changed`, `Fixed` i `Removed`. Po wydaniu sekcję przenosi się pod numer konkretnej wersji, na przykład `0.4.4`.

Starsze wpisy datowane pozostają bez zmiany, ponieważ nie mają wiarygodnie przypisanych numerów wersji.
