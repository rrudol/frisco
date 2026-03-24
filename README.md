# Frisco CLI

Unofficial CLI & MCP server for the [Frisco.pl](https://www.frisco.pl) grocery delivery API.

> **Disclaimer**: This is an independent, community project — not affiliated with Frisco. Use at your own risk.

## Features

- Interactive cart TUI (Bubble Tea)
- Product search with category filtering and nutrition info
- Delivery slot reservation
- Order history
- Batch cart additions from JSON shopping lists
- MCP server for AI assistant integration
- Session management (HAR import, cURL import, browser login)
- i18n support (English default, Polish available)

## Requirements

- Go `1.26+` (per `go.mod`)
- For `frisco auth login`: a locally installed browser supported by `chromedp` (e.g. Chrome/Chromium)

## Installation

```bash
go install github.com/rrudol/frisco/cmd/frisco@latest
```

Local build:

```bash
make build
./bin/frisco --help
```

## Quick start

1. Import session from a HAR file:

```bash
frisco har import --path ~/Downloads/www.frisco.pl.har
```

2. Or save session from a cURL command:

```bash
frisco session from-curl --curl "curl 'https://www.frisco.pl/app/commerce/api/v1/users/123/cart' -H 'authorization: Bearer ...' -H 'cookie: ...'"
```

3. Verify your session works:

```bash
frisco session verify
```

## Commands

```bash
frisco cart                    # Interactive TUI (list, +/-, remove, refresh)
frisco cart show
frisco cart add --product-id <id> --quantity 1
frisco cart add-batch --file list.json        # --dry-run for parse-only
frisco products search --search banana
frisco products search --search apple --category-id 18707
frisco products nutrition --product-id 4094
frisco reservation slots --days 2
frisco reservation reserve --date 2026-03-25 --from-time 06:00 --to-time 07:00
frisco orders list --all-pages
frisco account show
frisco xhr call --method GET --path-or-url "/app/commerce/api/v1/users/123/cart"
frisco mcp                     # Start MCP server (stdio)
```

### Batch cart from a shopping list

1. `frisco session verify` — make sure your session is valid.
2. Search for products: `frisco products search --search "phrase"` — optionally add `--category-id` to narrow results. Use `--format json` and pipe to `jq` to extract product IDs.
3. Build a JSON file: an array of `{ "product_id": "…", "quantity": n }` objects or `{"items":[…]}`. Template: [examples/cart-add-batch.example.json](examples/cart-add-batch.example.json).
4. `frisco cart add-batch --file list.json` — displays a cart summary after adding. Use `--dry-run` to validate the file without calling the API.

Helper script (search + SKU picker + JSON output): [scripts/build-cart-batch-from-searches.sh](scripts/build-cart-batch-from-searches.sh) with a sample search list in [scripts/weekly-shop-searches.txt](scripts/weekly-shop-searches.txt).

## Output format

By default the CLI outputs human-readable tables. Use `--format json` for machine-readable output:

```bash
frisco orders list --all-pages
frisco orders list --all-pages --format json
```

## Language (i18n)

The CLI defaults to English. Polish is available via `--lang pl` or the `FRISCO_LANG` environment variable:

```bash
frisco --lang pl --help
FRISCO_LANG=pl frisco cart show
```

Language priority:

1. `--lang` flag
2. `FRISCO_LANG` env var
3. `LC_ALL` / `LC_MESSAGES` / `LANG`
4. Fallback: `en`

## Session data

Session is stored locally at `~/.frisco-cli/session.json`.

Security notes:

- The file may contain tokens and session headers (`Authorization`, `Cookie`).
- Access to this file grants API access in your account's context.
- Do not run `frisco mcp` in untrusted or shared environments.

If an access token expires, the CLI will automatically attempt to refresh it. If refresh fails, re-import your session with `frisco session from-curl` or `frisco auth login`.

## Example payloads

- `shipping_address.example.json`
- `reservation_payload.example.json`
- [examples/cart-add-batch.example.json](examples/cart-add-batch.example.json)

## License

[MIT](LICENSE)
