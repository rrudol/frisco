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
- Session management (cURL import, browser login)


## Requirements

- Go `1.26+` (per `go.mod`)
- For `frisco session login`: a locally installed browser supported by `chromedp` (e.g. Chrome/Chromium)

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

1. Log in via browser (recommended):

```bash
frisco session login
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
frisco reservation cancel
frisco account show
frisco account orders list --all-pages
frisco session login               # Interactive browser login
frisco session refresh-token       # Refresh access token
frisco mcp                         # Start MCP server (stdio)
```

### Batch cart from a shopping list

1. `frisco session verify` — make sure your session is valid.
2. Search for products: `frisco products search --search "phrase"` — optionally add `--category-id` to narrow results. Use `--format json` and pipe to `jq` to extract product IDs.
3. Build a JSON file: an array of `{ "product_id": "…", "quantity": n }` objects or `{"items":[…]}`. Template: [examples/cart-add-batch.example.json](examples/cart-add-batch.example.json).
4. `frisco cart add-batch --file list.json` — displays a cart summary after adding. Use `--dry-run` to validate the file without calling the API.

## Category IDs

Using `--category-id` with `products search` / `products pick` narrows results and dramatically improves relevance. Full category tree: [categories.md](categories.md).

## Output format

By default the CLI outputs human-readable tables. Use `--format json` for machine-readable output:

```bash
frisco account orders list --all-pages
frisco account orders list --all-pages --format json
```

## MCP server — AI assistant integration

`frisco mcp` starts an [MCP](https://modelcontextprotocol.io) server over stdio, exposing cart, products, orders, reservations, and session tools to any compatible AI client.

### Claude Desktop

Download the latest `.mcpb` bundle for your platform from [Releases](https://github.com/rrudol/frisco/releases), then in Claude Desktop go to **Extensions → Advanced Settings → Install Extensions** and select the downloaded file.

**Login:** Ask Claude to log you in to Frisco (e.g. "zaloguj mnie do Frisco"). A Chrome window will open — log in on frisco.pl and navigate to your cart or account page. The session is captured automatically and saved for future use. Requires Chrome/Chromium.

<details>
<summary>Manual setup</summary>

Add to `~/Library/Application Support/Claude/claude_desktop_config.json` (macOS) or `%APPDATA%\Claude\claude_desktop_config.json` (Windows):

```json
{
  "mcpServers": {
    "frisco": {
      "command": "frisco",
      "args": ["mcp"]
    }
  }
}
```

If `frisco` is not in your `PATH`, use the absolute path to the binary (e.g. `"/Users/you/go/bin/frisco"`).

</details>

### Cursor

[![Install MCP Server](https://cursor.com/deeplink/mcp-install-dark.svg)](https://cursor.com/install-mcp?name=frisco&config=eyJjb21tYW5kIjoiZnJpc2NvIiwiYXJncyI6WyJtY3AiXX0=)

<details>
<summary>Manual setup</summary>

Add to `.cursor/mcp.json`:

```json
{
  "mcpServers": {
    "frisco": {
      "command": "frisco",
      "args": ["mcp"]
    }
  }
}
```

</details>

### Claude Code

```bash
claude mcp add frisco -- frisco mcp
```

<details>
<summary>Other MCP clients</summary>

Any client supporting stdio transport works:

```bash
frisco mcp
```

The server exposes tools for: product search, cart management, order history, delivery reservation, account info, and session management.

</details>

## Session data

Session is stored locally at `~/.frisco-cli/session.json`.

Security notes:

- The file may contain tokens and session headers (`Authorization`, `Cookie`).
- Access to this file grants API access in your account's context.
- Do not run `frisco mcp` in untrusted or shared environments.

If an access token expires, the CLI will automatically attempt to refresh it. If refresh fails, re-import your session with `frisco session from-curl` or `frisco session login`.

## Example payloads

- [examples/shipping-address.example.json](examples/shipping-address.example.json)
- [examples/reservation-payload.example.json](examples/reservation-payload.example.json)
- [examples/cart-add-batch.example.json](examples/cart-add-batch.example.json)

## License

[MIT](LICENSE)
