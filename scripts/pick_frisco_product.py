#!/usr/bin/env python3
"""
Wybiera najlepszy produkt z wyniku `frisco products search` dla danej frazy:
- dopasowanie nazwy (tokeny z frazy vs nazwa PL, tolerancja odmian),
- sensowna wielkość opakowania (grammage),
- najniższa cena za kg albo za litr (wg unitOfMeasure); dla „Piece” — najtańsza pasująca sztuka/opakowanie.

Wywołanie:
  FRISCO=/path/to/frisco python3 pick_frisco_product.py "fraza" [category_id]
  (category_id opcjonalnie — to samo co frisco products search --category-id, zawęża katalog)
Stdout: jedna linia JSON {"product_id","name_pl","category_id",...}
Exit 1 jeśli brak kandydatów.
"""
from __future__ import annotations

import json
import math
import os
import re
import subprocess
import sys
import unicodedata
from typing import Any


def fold(s: str) -> str:
    if not s:
        return ""
    s = unicodedata.normalize("NFD", s)
    s = "".join(c for c in s if unicodedata.category(c) != "Mn")
    return s.casefold()


def phrase_tokens(phrase: str) -> list[str]:
    phrase = phrase.strip()
    raw = re.split(r"[^\w\dąćęłńóśźżĄĆĘŁŃÓŚŹŻ]+", phrase, flags=re.I)
    return [t for t in raw if len(t) >= 2]


def name_words(name_l: str) -> list[str]:
    return [w for w in re.split(r"[^\w\dąćęłńóśźż]+", name_l, flags=re.I) if len(w) >= 2]


def token_matches_name(token: str, name_l: str, words: list[str]) -> bool:
    if len(token) < 2:
        return False
    if token in name_l:
        return True
    t = fold(token)
    for w in words:
        wf = fold(w)
        if t == wf:
            return True
        if len(t) >= 3 and (t in wf or wf in t):
            return True
        if len(t) >= 4 and wf.startswith(t[:4]):
            return True
        if len(wf) >= 4 and t.startswith(wf[:4]):
            return True
    return False


def match_score(phrase: str, name_pl: str) -> tuple[float, int]:
    """Zwraca (0..1), liczba trafionych tokenów."""
    if not name_pl:
        return 0.0, 0
    name_l = name_pl.strip().casefold()
    tokens = phrase_tokens(phrase)
    if not tokens:
        return 1.0, 0
    words = name_words(name_l)
    hits = sum(1 for t in tokens if token_matches_name(t.casefold(), name_l, words))
    return hits / len(tokens), hits


def price_of(p: dict[str, Any]) -> float | None:
    pr = p.get("price")
    if not isinstance(pr, dict):
        return None
    try:
        return float(pr.get("price"))
    except (TypeError, ValueError):
        return None


def pack_bonus_kg(g: float) -> float:
    """Preferuj opakowania ok. 0,35–1,5 kg; karz mikro-opakowania i worki XXL."""
    if g <= 0:
        return 0.0
    if 0.35 <= g <= 1.5:
        return 1.0
    if g < 0.35:
        return max(0.0, g / 0.35) * 0.85
    # duże opakowania: nadal OK, ale mniej preferowane
    return max(0.2, 1.0 - min(0.6, (g - 1.5) / 8.0))


def pack_bonus_litre(g: float) -> float:
    if g <= 0:
        return 0.0
    if 0.25 <= g <= 1.0:
        return 1.0
    if g < 0.25:
        return max(0.0, g / 0.25) * 0.85
    return max(0.25, 1.0 - min(0.5, (g - 1.0) / 5.0))


def score_candidate(phrase: str, item: dict[str, Any]) -> dict[str, Any] | None:
    prod = item.get("product")
    if not isinstance(prod, dict):
        return None
    name_obj = prod.get("name")
    name_pl = ""
    if isinstance(name_obj, dict):
        name_pl = str(name_obj.get("pl") or name_obj.get("en") or "")
    if not name_pl:
        return None
    if prod.get("isAvailable") is False:
        return None
    if prod.get("isStocked") is False:
        return None

    mscore, hits = match_score(phrase, name_pl)
    tokens = phrase_tokens(phrase)
    if tokens and hits == 0:
        return None

    pid = str(item.get("productId") or prod.get("productId") or prod.get("id") or "").strip()
    if not pid:
        return None

    price = price_of(prod)
    if price is None:
        return None

    uom = prod.get("unitOfMeasure") or ""
    g = prod.get("grammage")
    try:
        grammage = float(g) if g is not None else 0.0
    except (TypeError, ValueError):
        grammage = 0.0

    ppu: float | None = None
    ppu_label = ""
    if uom == "Kilogram" and grammage > 0:
        ppu = price / grammage
        ppu_label = "PLN/kg"
    elif uom == "Litre" and grammage > 0:
        ppu = price / grammage
        ppu_label = "PLN/l"
    elif uom == "Piece":
        # Porównujemy głównie cenę całego opakowania (szt./kpl.); kg z nazwy jest zawodne.
        ppu = float(price)
        ppu_label = "PLN/opak."
        if grammage > 0.05:
            est_kg = price / grammage
            ppu_label = f"PLN/opak (~{est_kg:.2f} PLN/kg wg grammage)"
    else:
        if grammage > 0:
            ppu = price / grammage
            ppu_label = "PLN/jedn."

    pack_b = 0.5
    if uom == "Kilogram":
        pack_b = pack_bonus_kg(grammage)
    elif uom == "Litre":
        pack_b = pack_bonus_litre(grammage)
    elif uom == "Piece":
        pack_b = 0.7 if 0.4 <= grammage <= 2.0 else 0.5

    big_penalty = 0.0
    low = name_pl.casefold()
    if any(x in low for x in ("zestaw", "pakiet", "multipack", " x ", "opakowanie zbiorcze")):
        big_penalty = 0.15

    if uom == "Piece":
        sort_ppu = float(price)
    else:
        sort_ppu = ppu if ppu is not None else float("inf")

    return {
        "product_id": pid,
        "name_pl": name_pl,
        "unit": uom,
        "grammage": grammage,
        "price": price,
        "price_per_unit": ppu,
        "ppu_label": ppu_label,
        "match": mscore,
        "hits": hits,
        "pack_b": pack_b,
        "big_penalty": big_penalty,
        "_sort": (
            -mscore,
            big_penalty,
            -pack_b,
            sort_ppu,
            grammage,
        ),
    }


def pick_best(phrase: str, products: list[dict[str, Any]]) -> dict[str, Any] | None:
    scored: list[dict[str, Any]] = []
    for item in products:
        if not isinstance(item, dict):
            continue
        s = score_candidate(phrase, item)
        if s:
            scored.append(s)
    if not scored:
        return None
    scored.sort(key=lambda x: x["_sort"])
    best = scored[0]
    ppu = best["price_per_unit"]
    ppu_r = round(float(ppu), 2) if ppu is not None else None
    note = f"match={best['match']:.2f} {best['ppu_label']}={ppu_r} opak={round(best['grammage'], 3)}{best['unit'][:1] if best['unit'] else ''}"
    out = {
        "product_id": best["product_id"],
        "name_pl": best["name_pl"],
        "unit": best["unit"],
        "grammage": round(best["grammage"], 4),
        "price": round(best["price"], 2),
        "price_per_unit": ppu_r,
        "note": note,
    }
    return out


def main() -> None:
    if len(sys.argv) < 2:
        print('Użycie: pick_frisco_product.py "fraza" [category_id]', file=sys.stderr)
        sys.exit(2)
    phrase = sys.argv[1]
    category = (sys.argv[2] if len(sys.argv) > 2 else os.environ.get("FRISCO_CATEGORY_ID", "")).strip()
    frisco = os.environ.get("FRISCO", "frisco")
    cmd = [frisco, "--format", "json", "products", "search", "--search", phrase]
    if category:
        cmd.extend(["--category-id", category])
    r = subprocess.run(
        cmd,
        capture_output=True,
        text=True,
        timeout=90,
    )
    if r.returncode != 0:
        print(r.stderr or r.stdout, file=sys.stderr)
        sys.exit(1)
    try:
        data = json.loads(r.stdout)
    except json.JSONDecodeError as e:
        print(e, file=sys.stderr)
        sys.exit(1)
    products = data.get("products")
    if not isinstance(products, list) or not products:
        sys.exit(1)
    best = pick_best(phrase, products)
    if not best:
        sys.exit(1)
    if category:
        best["category_id"] = category
    print(json.dumps(best, ensure_ascii=False))


if __name__ == "__main__":
    main()
