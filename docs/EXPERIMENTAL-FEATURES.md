# Funkcje eksperymentalne

Panel funkcji eksperymentalnych jest dostepny wylacznie w buildach developerskich.

- Backend ustawia `experimental_features` w `/api/status` na podstawie linkerowej zmiennej `BuildChannel`.
- `BuildChannel` domyślnie ma wartość `main`; tylko workflow `channel-release.yml` ustawia ją na `dev`.
- Wersje `main` muszą zwracać `false`; frontend traktuje brak tej wartości również jako `false`.
- Włączenie opcji jest sesyjne i wymaga potwierdzenia ostrzeżenia. Nie jest zapisywane w konfiguracji użytkownika.
- UPRdirect jest pierwszą pozycją tej listy. Mapa, tryb UPRD w MHEARD i karta `Bikon` są odblokowywane razem z tą pozycją.

Przy dodawaniu kolejnych eksperymentów należy dopisać je do listy w `internal/web/static/index.html` oraz zachować blokadę po stronie buildu `main`.
