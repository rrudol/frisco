#!/usr/bin/env bash
# Uses frisco + ~/.frisco-cli/session. Linia: categoryId|fraza|ilość lub fraza|ilość.
# categoryId = frisco products search --category-id (zawęża katalog — patrz nagłówek listy).
# pick_frisco_product.py dobiera SKU w obrębie wyników.
# Review SKUs (skład, sól, cukier) before: frisco cart add-batch --file ... --dry-run
#
# Nie spamuj API: domyślnie pauza między wyszukiwaniami (FRISCO_SEARCH_DELAY_SEC, domyślnie 2).
# Nie uruchamiaj wielu kopii skryptu równolegle.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
FRISCO="${FRISCO:-$ROOT/bin/frisco}"
LIST="${1:-$ROOT/scripts/weekly-shop-searches.txt}"
OUT="${2:-$ROOT/examples/weekly-diet-cart.generated.json}"
# Seconds to sleep after each search (rate limit friendly).
DELAY_SEC="${FRISCO_SEARCH_DELAY_SEC:-2}"

if [[ ! -x "$FRISCO" ]]; then
  echo "Run: make build" >&2
  exit 1
fi
PICK_PY="$ROOT/scripts/pick_frisco_product.py"
if [[ ! -f "$PICK_PY" ]]; then
  echo "Brak: $PICK_PY" >&2
  exit 1
fi
if [[ ! -f "$LIST" ]]; then
  echo "Missing list: $LIST" >&2
  exit 1
fi

tmp="$(mktemp)"
trap 'rm -f "$tmp"' EXIT

while IFS= read -r line || [[ -n "$line" ]]; do
  [[ "$line" =~ ^[[:space:]]*# ]] && continue
  [[ -z "${line// }" ]] && continue
  line="${line//$'\r'/}"
  category=""
  phrase=""
  qty=""
  # Bez zwykłego read a|b|c: nadmiarowe | na końcu lini trafiałyby do qty jako "1|||".
  if [[ "$line" =~ ^([0-9]+)\|(.+)\|([0-9]+)$ ]]; then
    category="${BASH_REMATCH[1]}"
    phrase="${BASH_REMATCH[2]}"
    qty="${BASH_REMATCH[3]}"
  elif [[ "$line" =~ ^([^|]+)\|([0-9]+)$ ]]; then
    phrase="${BASH_REMATCH[1]}"
    qty="${BASH_REMATCH[2]}"
  else
    echo "WARN: zly format linii (oczekiwane: cat|fraza|ilosc lub fraza|ilosc): $line" >&2
    continue
  fi
  phrase="$(printf '%s' "$phrase" | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
  qty="$(printf '%s' "$qty" | tr -d '\r\n' | sed 's/^[[:space:]]*//;s/[[:space:]]*$//')"
  [[ -z "$qty" ]] && qty=1
  if ! [[ "$qty" =~ ^[1-9][0-9]*$ ]]; then
    echo "WARN: zla ilosc $qty dla $phrase - ustawiam 1" >&2
    qty=1
  fi

  export FRISCO
  if [[ -n "$category" ]]; then
    pick_json="$(python3 "$PICK_PY" "$phrase" "$category" 2>/dev/null)" || pick_json=""
  else
    pick_json="$(python3 "$PICK_PY" "$phrase" 2>/dev/null)" || pick_json=""
  fi
  pid="$(echo "$pick_json" | jq -r '.product_id // empty')"
  if [[ -z "$pid" || "$pid" == "null" ]]; then
    echo "WARN: brak wyboru: $phrase" >&2
  else
    pl="$(echo "$pick_json" | jq -r '.name_pl // empty')"
    note="$(echo "$pick_json" | jq -r '.note // empty')"
    echo "$phrase${category:+ [cat:$category]} → [$pid] $pl ($note)" >&2
    jq -nc --arg id "$pid" --argjson q "$qty" '{product_id:$id, quantity:$q}' >>"$tmp"
  fi
  # Pauza po każdym requeście (sukces lub pusty wynik), żeby nie obciążać API.
  sleep "$DELAY_SEC"
done <"$LIST"

if [[ ! -s "$tmp" ]]; then
  echo "Brak żadnego productId — sprawdź sesję: frisco session verify" >&2
  exit 1
fi

jq -s '{items: .}' "$tmp" >"$OUT"
echo "Zapisano: $OUT ($(jq '.items | length' "$OUT") pozycji)" >&2
