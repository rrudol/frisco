# Frisco CLI (Go)

CLI do obsługi Frisco API: sesja, HAR/XHR, produkty, koszyk, rezerwacje, konto, zamówienia, auth.

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

## Najważniejsze komendy

```bash
frisco cart                 # TUI Bubble Tea (lista, +/-, usuń, odśwież)
frisco cart show
frisco products search --search ban
frisco products nutrition --product-id 4094
frisco reservation slots --days 2
frisco reservation reserve --date 2026-03-25 --from-time 06:00 --to-time 07:00
frisco orders list --all-pages
frisco xhr call --method GET --path-or-url "/app/commerce/api/v1/users/123/cart"
```

## Dane sesji

Sesja jest zapisywana lokalnie w:

- `~/.frisco-cli/session.json`

Jeśli access token wygaśnie, CLI przy `401` spróbuje automatycznie odświeżyć go przez `refresh_token` i ponowi request raz.
Jeśli nie ma `refresh_token` albo refresh też się nie powiedzie, ponownie użyj `frisco session from-curl` albo `frisco auth login`.

## Przykładowe payloady

- `shipping_address.example.json`
- `reservation_payload.example.json`
