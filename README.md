# Frisco CLI (Go)

CLI do obsługi Frisco API: sesja, HAR/XHR, produkty, koszyk, rezerwacje, konto, zamówienia, auth.

## Ważne

- To jest projekt nieoficjalny, niezależny od Frisco.
- Korzystanie z API i automatyzacji odbywa się na własną odpowiedzialność użytkownika.

## Wymagania

- Go `1.26+` (zgodnie z `go.mod`)
- Dla `frisco auth login`: lokalnie zainstalowana przeglądarka wspierana przez `chromedp` (np. Chrome/Chromium)

## Instalacja

Po publikacji repo:

```bash
go install github.com/rrudol/frisco/cmd/frisco@latest
```

W terminalu dostaniesz binarkę pod nazwą `frisco`.

Lokalnie:

```bash
make build
./bin/frisco --help
./bin/frisco mcp
```

Szybkie uruchomienie bez budowania:

```bash
make run
```

## Szybki start

1. Import endpointów XHR z HAR:

```bash
frisco har import --path "/Users/rafal/Downloads/www.frisco.pl.har"
```

2. Zapis sesji z cURL:

```bash
frisco session from-curl --curl "curl 'https://www.frisco.pl/app/commerce/api/v1/users/123/cart' -H 'authorization: Bearer ...' -H 'cookie: ...'"
```

3. Sprawdzenie sesji:

```bash
frisco session show
```

4. (Zalecane przed masowym dodawaniem do koszyka) Weryfikacja, że token i `user_id` działają z API:

```bash
frisco session verify
```

## Najważniejsze komendy

```bash
frisco cart                 # TUI Bubble Tea (lista, +/-, usuń, odśwież)
frisco cart show
frisco cart add --product-id <id> --quantity 1
frisco cart add-batch --file path/to/list.json   # --dry-run tylko parsowanie
frisco products search --search ban
frisco products search --search jabłko --category-id 18707
frisco products nutrition --product-id 4094
frisco reservation slots --days 2
frisco reservation reserve --date 2026-03-25 --from-time 06:00 --to-time 07:00
frisco orders list --all-pages
frisco xhr call --method GET --path-or-url "/app/commerce/api/v1/users/123/cart"
frisco mcp
```

### Koszyk z listy zakupów (np. jadłospis)

1. `frisco session verify` — upewnij się, że sesja jest ważna.
2. Dla każdej pozycji z listy: `frisco products search --search "fraza"` — opcjonalnie `--category-id <Frisco categoryId>`, żeby odfiltrować np. napoje przy owocach (np. `18703` Warzywa i owoce, `18707` Jabłka). Dalej `--format json` + `jq` po `productId`. Skład produktu sprawdzasz sam (CLI nie filtruje po diecie).
3. Zbuduj plik JSON: tablica obiektów `{ "product_id": "…", "quantity": n }` albo obiekt `{"items":[…]}`. Szablon: [examples/cart-add-batch.example.json](examples/cart-add-batch.example.json).
4. `frisco cart add-batch --file list.json` — po dodaniu zobaczysz podsumowanie koszyka (tryb `table`). `frisco cart add-batch --file list.json --dry-run` sprawdza plik bez wywołań API.

Skrypt pomocniczy (wyszukiwanie + wybór SKU + JSON): [scripts/build-cart-batch-from-searches.sh](scripts/build-cart-batch-from-searches.sh) i lista `categoryId|fraza|ilość` w [scripts/weekly-shop-searches.txt](scripts/weekly-shop-searches.txt).

## Format wyjścia

Domyślnie CLI pokazuje wynik w formacie czytelnym dla człowieka (`table`).
JSON dostaniesz tylko na żądanie:

```bash
frisco orders list --all-pages
frisco orders list --all-pages --format json
```

Dostępne wartości:

- `--format table` (domyślnie)
- `--format json`

## Język (i18n)

CLI wspiera dwa języki:

- `en` (domyślny)
- `pl`

Ustawienie języka:

```bash
frisco --lang en --help
frisco --lang pl --help
```

Możesz też ustawić zmienną środowiskową:

```bash
export FRISCO_LANG=pl
frisco --help
```

Notka: zmiana przez zmienną środowiskową:

- tylko dla jednego wywołania:

```bash
FRISCO_LANG=pl frisco cart show
```

- na stałe (np. `zsh`), dopisz do `~/.zshrc`:

```bash
export FRISCO_LANG=pl
```

Priorytet wyboru języka:

1. flaga `--lang`
2. `FRISCO_LANG`
3. `LC_ALL` / `LC_MESSAGES` / `LANG`
4. fallback `en`

## Dane sesji

Sesja jest zapisywana lokalnie w:

- `~/.frisco-cli/session.json`

Uwagi bezpieczeństwa:

- plik może zawierać tokeny i nagłówki sesyjne (np. `Authorization`, `Cookie`);
- dostęp do tego pliku daje dostęp do API w kontekście Twojego konta;
- nie uruchamiaj `frisco mcp` w niezaufanym lub współdzielonym środowisku.

Jeśli access token wygaśnie, CLI przy `401` spróbuje automatycznie odświeżyć go przez `refresh_token` i ponowi request raz.
Jeśli nie ma `refresh_token` albo refresh też się nie powiedzie, ponownie użyj `frisco session from-curl` albo `frisco auth login`.

## Przykładowe payloady

- `shipping_address.example.json`
- `reservation_payload.example.json`
- [examples/cart-add-batch.example.json](examples/cart-add-batch.example.json) — lista produktów do `cart add-batch`
