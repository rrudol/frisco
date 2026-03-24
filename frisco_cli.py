#!/usr/bin/env python3
import argparse
import datetime as dt
import json
import re
import shlex
import sys
from dataclasses import dataclass
from pathlib import Path
from typing import Any, Dict, List, Optional, Tuple
from urllib.parse import parse_qsl, urlencode, urljoin, urlparse
from urllib.request import Request, urlopen
from urllib.error import HTTPError, URLError


DEFAULT_BASE_URL = "https://www.frisco.pl"
DEFAULT_HAR_PATH = "/Users/rafal/Downloads/www.frisco.pl.har"
SESSION_DIR = Path.home() / ".frisco-cli"
SESSION_FILE = SESSION_DIR / "session.json"


def ensure_session_dir() -> None:
    SESSION_DIR.mkdir(parents=True, exist_ok=True)


def load_session() -> Dict[str, Any]:
    if not SESSION_FILE.exists():
        return {
            "base_url": DEFAULT_BASE_URL,
            "headers": {},
            "token": None,
            "user_id": None,
            "endpoints": [],
            "har_path": None,
        }
    with SESSION_FILE.open("r", encoding="utf-8") as f:
        return json.load(f)


def save_session(data: Dict[str, Any]) -> None:
    ensure_session_dir()
    with SESSION_FILE.open("w", encoding="utf-8") as f:
        json.dump(data, f, ensure_ascii=False, indent=2)


def normalize_path(path: str) -> str:
    path = re.sub(r"/\d+(?=/|$)", "/{id}", path)
    path = re.sub(
        r"/[0-9a-fA-F]{8}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{4}-[0-9a-fA-F]{12}(?=/|$)",
        "/{uuid}",
        path,
    )
    path = re.sub(r"/\d{4}/\d{1,2}/\d{1,2}(?=/|$)", "/{yyyy}/{m}/{d}", path)
    return path


def parse_har_xhr(har_path: str) -> List[Dict[str, Any]]:
    with open(har_path, "r", encoding="utf-8") as f:
        har = json.load(f)
    entries = har.get("log", {}).get("entries", [])
    out: List[Dict[str, Any]] = []

    for e in entries:
        resource_type = e.get("_resourceType") or e.get("resourceType")
        if resource_type != "xhr":
            continue
        req = e.get("request", {})
        raw_url = req.get("url", "")
        parsed = urlparse(raw_url)
        out.append(
            {
                "method": req.get("method", "GET").upper(),
                "host": f"{parsed.scheme}://{parsed.netloc}",
                "path": parsed.path or "/",
                "path_template": normalize_path(parsed.path or "/"),
                "has_query": bool(parsed.query),
                "url": raw_url,
            }
        )

    unique: Dict[Tuple[str, str, str], Dict[str, Any]] = {}
    for row in out:
        key = (row["method"], row["host"], row["path_template"])
        if key not in unique:
            unique[key] = row
    return sorted(unique.values(), key=lambda r: (r["host"], r["path_template"], r["method"]))


def parse_json_or_kv_data(raw: Optional[str]) -> Optional[Any]:
    if not raw:
        return None
    raw = raw.strip()
    if not raw:
        return None
    if raw.startswith("{") or raw.startswith("["):
        return json.loads(raw)
    kv_pairs = dict(parse_qsl(raw))
    return kv_pairs if kv_pairs else raw


@dataclass
class CurlData:
    method: str
    url: str
    headers: Dict[str, str]
    body: Optional[str]


def parse_curl(curl_command: str) -> CurlData:
    tokens = shlex.split(curl_command)
    if not tokens:
        raise ValueError("Pusty curl.")
    if tokens[0] != "curl":
        raise ValueError("Komenda musi zaczynać się od 'curl'.")

    method = "GET"
    url = ""
    headers: Dict[str, str] = {}
    body = None

    i = 1
    while i < len(tokens):
        token = tokens[i]
        nxt = tokens[i + 1] if i + 1 < len(tokens) else None

        if token in ("-X", "--request") and nxt:
            method = nxt.upper()
            i += 2
            continue
        if token in ("-H", "--header") and nxt:
            if ":" in nxt:
                k, v = nxt.split(":", 1)
                headers[k.strip()] = v.strip()
            i += 2
            continue
        if token in ("--data", "--data-raw", "--data-binary", "-d") and nxt:
            body = nxt
            if method == "GET":
                method = "POST"
            i += 2
            continue
        if token in ("--url",) and nxt:
            url = nxt
            i += 2
            continue
        if token.startswith("http://") or token.startswith("https://"):
            url = token
            i += 1
            continue

        i += 1

    if not url:
        raise ValueError("Nie udało się znaleźć URL w curl.")

    return CurlData(method=method, url=url, headers=headers, body=body)


def extract_token(headers: Dict[str, str]) -> Optional[str]:
    for key, value in headers.items():
        if key.lower() == "authorization" and value.lower().startswith("bearer "):
            return value.split(" ", 1)[1].strip()
    return None


def extract_user_id(url: str) -> Optional[str]:
    m = re.search(r"/users/(\d+)", url)
    if m:
        return m.group(1)
    return None


def make_url(base_url: str, path_or_url: str) -> str:
    if path_or_url.startswith("http://") or path_or_url.startswith("https://"):
        return path_or_url
    return urljoin(base_url, path_or_url)


def request_json(
    session: Dict[str, Any],
    method: str,
    path_or_url: str,
    query: Optional[List[str]] = None,
    data: Optional[Any] = None,
    data_format: str = "json",
    extra_headers: Optional[Dict[str, str]] = None,
) -> Any:
    base_url = session.get("base_url") or DEFAULT_BASE_URL
    url = make_url(base_url, path_or_url)
    params: List[Tuple[str, str]] = []
    if query:
        for p in query:
            if "=" not in p:
                raise ValueError(f"Zły parametr query: {p}. Oczekiwane key=value")
            k, v = p.split("=", 1)
            params.append((k, v))
    if params:
        sep = "&" if "?" in url else "?"
        url = f"{url}{sep}{urlencode(params, doseq=True)}"

    headers = dict(session.get("headers", {}))
    token = session.get("token")
    if token and "Authorization" not in headers and "authorization" not in headers:
        headers["Authorization"] = f"Bearer {token}"
    if extra_headers:
        headers.update(extra_headers)

    body_bytes = None
    if data is not None:
        if data_format == "json":
            body_bytes = json.dumps(data, ensure_ascii=False).encode("utf-8")
            if "Content-Type" not in headers and "content-type" not in headers:
                headers["Content-Type"] = "application/json"
        elif data_format == "form":
            if isinstance(data, dict):
                body_bytes = urlencode(data).encode("utf-8")
            elif isinstance(data, str):
                body_bytes = data.encode("utf-8")
            else:
                raise ValueError("Dla data_format=form podaj dict albo string.")
            if "Content-Type" not in headers and "content-type" not in headers:
                headers["Content-Type"] = "application/x-www-form-urlencoded"
        elif data_format == "raw":
            if isinstance(data, str):
                body_bytes = data.encode("utf-8")
            else:
                raise ValueError("Dla data_format=raw podaj string.")
        else:
            raise ValueError("Nieobsługiwany data_format. Użyj: json, form, raw.")

    req = Request(url=url, method=method.upper(), data=body_bytes, headers=headers)
    try:
        with urlopen(req, timeout=30) as resp:
            content_type = resp.headers.get("Content-Type", "")
            raw = resp.read().decode("utf-8", errors="replace")
            if "application/json" in content_type:
                return json.loads(raw) if raw else {}
            return {"status": resp.status, "body": raw}
    except HTTPError as e:
        error_text = e.read().decode("utf-8", errors="replace")
        msg = {
            "status": e.code,
            "reason": e.reason,
            "url": url,
            "body": error_text,
        }
        raise RuntimeError(json.dumps(msg, ensure_ascii=False, indent=2)) from e
    except URLError as e:
        raise RuntimeError(f"Błąd połączenia: {e}") from e


def print_json(data: Any) -> None:
    print(json.dumps(data, ensure_ascii=False, indent=2))


def cmd_session_from_curl(args: argparse.Namespace) -> None:
    session = load_session()
    curl_data = parse_curl(args.curl)

    for k, v in curl_data.headers.items():
        kl = k.lower()
        if kl in {
            "authorization",
            "content-type",
            "cookie",
            "x-api-version",
            "x-requested-with",
            "accept",
            "origin",
            "referer",
        }:
            session.setdefault("headers", {})[k] = v

    token = extract_token(curl_data.headers)
    if token:
        session["token"] = token
    refresh_token = extract_refresh_token_from_curl_body(curl_data.body) or extract_refresh_token_from_cookie(
        curl_data.headers.get("cookie") or curl_data.headers.get("Cookie")
    )
    if refresh_token:
        session["refresh_token"] = refresh_token

    inferred_user_id = extract_user_id(curl_data.url)
    if inferred_user_id:
        session["user_id"] = inferred_user_id

    parsed = urlparse(curl_data.url)
    if parsed.scheme and parsed.netloc:
        session["base_url"] = f"{parsed.scheme}://{parsed.netloc}"

    save_session(session)
    print("Zapisano sesję na podstawie curl.")
    print_json(
        {
            "base_url": session.get("base_url"),
            "user_id": session.get("user_id"),
            "token_saved": bool(session.get("token")),
            "headers_saved": sorted(list(session.get("headers", {}).keys())),
        }
    )


def cmd_session_show(_: argparse.Namespace) -> None:
    session = load_session()
    safe = dict(session)
    if safe.get("token"):
        safe["token"] = "***"
    if safe.get("refresh_token"):
        safe["refresh_token"] = "***"
    if "headers" in safe and isinstance(safe["headers"], dict):
        redacted = {}
        for k, v in safe["headers"].items():
            if k.lower() in {"authorization", "cookie"}:
                redacted[k] = "***"
            else:
                redacted[k] = v
        safe["headers"] = redacted
    print_json(safe)


def cmd_har_import(args: argparse.Namespace) -> None:
    session = load_session()
    endpoints = parse_har_xhr(args.path)
    session["endpoints"] = endpoints
    session["har_path"] = args.path

    for ep in endpoints:
        uid = extract_user_id(ep["url"])
        if uid:
            session["user_id"] = uid
            break

    save_session(session)
    print(f"Zaimportowano XHR: {len(endpoints)} unikalnych endpointów.")


def cmd_xhr_list(args: argparse.Namespace) -> None:
    session = load_session()
    endpoints = session.get("endpoints", [])
    if not endpoints:
        print("Brak endpointów w sesji. Uruchom: har import")
        return

    filtered = endpoints
    if args.contains:
        needle = args.contains.lower()
        filtered = [
            ep
            for ep in endpoints
            if needle in ep["path_template"].lower() or needle in ep["method"].lower()
        ]

    for ep in filtered:
        q = "?" if ep.get("has_query") else ""
        print(f'{ep["method"]:6} {ep["path_template"]}{q}')
    print(f"\nRazem: {len(filtered)}")


def cmd_xhr_call(args: argparse.Namespace) -> None:
    session = load_session()
    payload = parse_json_or_kv_data(args.data)
    data_format = args.data_format
    if payload is not None and data_format == "auto":
        if isinstance(args.data, str) and args.data.strip().startswith(("{", "[")):
            data_format = "json"
        elif isinstance(args.data, str) and "=" in args.data:
            data_format = "form"
        else:
            data_format = "raw"
    headers = {}
    for h in args.header or []:
        if ":" not in h:
            raise ValueError(f"Zły nagłówek: {h}. Oczekiwane Key: Value")
        k, v = h.split(":", 1)
        headers[k.strip()] = v.strip()

    result = request_json(
        session=session,
        method=args.method,
        path_or_url=args.path_or_url,
        query=args.query,
        data=payload,
        data_format=data_format,
        extra_headers=headers or None,
    )
    print_json(result)


def require_user_id(session: Dict[str, Any], explicit_user_id: Optional[str]) -> str:
    uid = explicit_user_id or session.get("user_id")
    if not uid:
        raise ValueError(
            "Brak user_id. Wklej curl z endpointem /users/{id}/... przez 'session from-curl' "
            "albo podaj --user-id."
        )
    return str(uid)


def cmd_products_search(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    path = f"/app/commerce/api/v1/users/{user_id}/offer/products/query"
    query = [
        "purpose=Listing",
        f"pageIndex={args.page_index}",
        f"search={args.search}",
        "includeFacets=true",
        f"deliveryMethod={args.delivery_method}",
        f"pageSize={args.page_size}",
        "language=pl",
        "disableAutocorrect=false",
    ]
    result = request_json(session, "GET", path, query=query)
    print_json(result)


def cmd_products_by_ids(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    path = f"/app/commerce/api/v1/users/{user_id}/offer/products"
    query = [f"productIds={pid}" for pid in args.product_id]
    result = request_json(session, "GET", path, query=query)
    print_json(result)


def cmd_cart_show(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    path = f"/app/commerce/api/v1/users/{user_id}/cart"
    result = request_json(session, "GET", path)
    print_json(result)


def cmd_cart_add(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    path = f"/app/commerce/api/v1/users/{user_id}/cart"
    body = {"products": [{"productId": str(args.product_id), "quantity": int(args.quantity)}]}
    result = request_json(session, "PUT", path, data=body)
    print_json(result)


def cmd_cart_remove(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    path = f"/app/commerce/api/v1/users/{user_id}/cart"
    body = {"products": [{"productId": str(args.product_id), "quantity": 0}]}
    result = request_json(session, "PUT", path, data=body)
    print_json(result)


def cmd_reservation_delivery_options(args: argparse.Namespace) -> None:
    session = load_session()
    path = "/app/commerce/api/v1/calendar/delivery-payment"
    query = [f"postcode={args.postcode}"]
    result = request_json(session, "GET", path, query=query)
    print_json(result)


def load_json_file(path: Optional[str]) -> Optional[Any]:
    if not path:
        return None
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def cmd_reservation_calendar(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    shipping_address = load_json_file(args.shipping_address_file)
    if not isinstance(shipping_address, dict):
        raise ValueError("Plik z adresem musi zawierać obiekt JSON.")

    body = {"shippingAddress": shipping_address}
    if args.date:
        date_parts = args.date.split("-")
        if len(date_parts) != 3:
            raise ValueError("Data musi mieć format YYYY-M-D lub YYYY-MM-DD")
        y, m, d = date_parts
        path = f"/app/commerce/api/v2/users/{user_id}/calendar/Van/{int(y)}/{int(m)}/{int(d)}"
    else:
        path = f"/app/commerce/api/v2/users/{user_id}/calendar/Van"
    result = request_json(session, "POST", path, data=body)
    print_json(result)


def get_shipping_address_from_account(session: Dict[str, Any], user_id: str) -> Dict[str, Any]:
    path = f"/app/commerce/api/v1/users/{user_id}/addresses/shipping-addresses"
    data = request_json(session, "GET", path)
    if not isinstance(data, list) or not data:
        raise ValueError("Brak zapisanych adresów użytkownika.")

    # Prefer default/current address if available.
    preferred = None
    for item in data:
        if not isinstance(item, dict):
            continue
        if item.get("isDefault") or item.get("isCurrent") or item.get("isSelected"):
            preferred = item
            break
    chosen = preferred or data[0]

    shipping_address = chosen.get("shippingAddress") if isinstance(chosen, dict) else None
    if isinstance(shipping_address, dict):
        return shipping_address

    # Fallback when API returns address fields at root.
    if isinstance(chosen, dict):
        return chosen
    raise ValueError("Nie udało się odczytać adresu dostawy z konta.")


def extract_delivery_windows(data: Any) -> List[Dict[str, Any]]:
    windows: List[Dict[str, Any]] = []

    def walk(obj: Any) -> None:
        if isinstance(obj, dict):
            starts = obj.get("startsAt")
            ends = obj.get("endsAt")
            method = obj.get("deliveryMethod")
            warehouse = obj.get("warehouse")
            closes = obj.get("closesAt")
            final_at = obj.get("finalAt")
            if starts and ends:
                windows.append(
                    {
                        "startsAt": starts,
                        "endsAt": ends,
                        "deliveryMethod": method,
                        "warehouse": warehouse,
                        "closesAt": closes,
                        "finalAt": final_at,
                    }
                )
            for value in obj.values():
                walk(value)
        elif isinstance(obj, list):
            for item in obj:
                walk(item)

    walk(data)

    unique = {}
    for w in windows:
        key = (w.get("startsAt"), w.get("endsAt"), w.get("deliveryMethod"), w.get("warehouse"))
        unique[key] = w
    return sorted(unique.values(), key=lambda w: str(w.get("startsAt")))


def cmd_reservation_slots(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)

    if args.shipping_address_file:
        shipping_address = load_json_file(args.shipping_address_file)
        if not isinstance(shipping_address, dict):
            raise ValueError("Plik z adresem musi zawierać obiekt JSON.")
    else:
        shipping_address = get_shipping_address_from_account(session, user_id)

    if args.start_date:
        base_date = dt.date.fromisoformat(args.start_date)
    else:
        base_date = dt.date.today()

    all_days: Dict[str, Any] = {}
    pretty: List[Dict[str, Any]] = []
    for i in range(args.days):
        d = base_date + dt.timedelta(days=i)
        path = f"/app/commerce/api/v2/users/{user_id}/calendar/Van/{d.year}/{d.month}/{d.day}"
        day_data = request_json(session, "POST", path, data={"shippingAddress": shipping_address})
        day_key = d.isoformat()
        all_days[day_key] = day_data
        windows = extract_delivery_windows(day_data)
        pretty.append({"date": day_key, "slots": windows})

    if args.raw:
        print_json(all_days)
        return
    print_json({"days": pretty})


def extract_reservable_windows(data: Any) -> List[Dict[str, Any]]:
    windows: List[Dict[str, Any]] = []

    def walk(obj: Any) -> None:
        if isinstance(obj, dict):
            starts = obj.get("startsAt")
            ends = obj.get("endsAt")
            method = obj.get("deliveryMethod")
            warehouse = obj.get("warehouse")
            if starts and ends and method and warehouse:
                # Keep full window object from API for reservation payload.
                windows.append(obj)
            for value in obj.values():
                walk(value)
        elif isinstance(obj, list):
            for item in obj:
                walk(item)

    walk(data)
    unique = {}
    for w in windows:
        key = (w.get("startsAt"), w.get("endsAt"), w.get("deliveryMethod"), w.get("warehouse"))
        unique[key] = w
    return sorted(unique.values(), key=lambda w: str(w.get("startsAt")))


def _date_and_hhmm_from_iso(ts: str) -> Tuple[str, str]:
    if "T" not in ts:
        return ts, ""
    date_part, time_part = ts.split("T", 1)
    hhmm = time_part[:5]
    return date_part, hhmm


def cmd_reservation_reserve(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    target_date = dt.date.fromisoformat(args.date)
    from_hhmm = args.from_time
    to_hhmm = args.to_time

    if args.shipping_address_file:
        shipping_address = load_json_file(args.shipping_address_file)
        if not isinstance(shipping_address, dict):
            raise ValueError("Plik z adresem musi zawierać obiekt JSON.")
    else:
        shipping_address = get_shipping_address_from_account(session, user_id)

    cal_path = (
        f"/app/commerce/api/v2/users/{user_id}/calendar/Van/"
        f"{target_date.year}/{target_date.month}/{target_date.day}"
    )
    day_data = request_json(session, "POST", cal_path, data={"shippingAddress": shipping_address})
    windows = extract_reservable_windows(day_data)
    if not windows:
        raise ValueError("Brak dostępnych slotów rezerwacji dla podanej daty.")

    selected: Optional[Dict[str, Any]] = None
    for w in windows:
        starts_at = str(w.get("startsAt", ""))
        ends_at = str(w.get("endsAt", ""))
        d1, h1 = _date_and_hhmm_from_iso(starts_at)
        d2, h2 = _date_and_hhmm_from_iso(ends_at)
        if d1 == args.date and d2 == args.date and h1 == from_hhmm and h2 == to_hhmm:
            selected = w
            break

    if selected is None:
        possible = []
        for w in windows:
            starts_at = str(w.get("startsAt", ""))
            ends_at = str(w.get("endsAt", ""))
            d1, h1 = _date_and_hhmm_from_iso(starts_at)
            d2, h2 = _date_and_hhmm_from_iso(ends_at)
            if d1 == args.date and d2 == args.date:
                possible.append(f"{h1}-{h2}")
        raise ValueError(
            f"Nie znaleziono slotu {from_hhmm}-{to_hhmm} dla {args.date}. "
            f"Dostępne: {', '.join(possible)}"
        )

    payload = {
        "extendedRange": None,
        "deliveryWindow": selected,
        "shippingAddress": shipping_address,
    }
    reserve_path = f"/app/commerce/api/v2/users/{user_id}/cart/reservation"
    result = request_json(session, "POST", reserve_path, data=payload)
    print_json(
        {
            "reserved": True,
            "slot": {
                "startsAt": selected.get("startsAt"),
                "endsAt": selected.get("endsAt"),
                "deliveryMethod": selected.get("deliveryMethod"),
                "warehouse": selected.get("warehouse"),
            },
            "apiResponse": result,
        }
    )


def cmd_reservation_plan(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    payload = load_json_file(args.payload_file)
    if not isinstance(payload, dict):
        raise ValueError("Plik payload musi zawierać obiekt JSON.")
    path = f"/app/commerce/api/v2/users/{user_id}/cart/reservation"
    result = request_json(session, "POST", path, data=payload)
    print_json(result)


def cmd_reservation_status(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    path = f"/app/commerce/api/v1/users/{user_id}/orders"
    query = [f"pageIndex={args.page_index}", f"pageSize={args.page_size}"]
    result = request_json(session, "GET", path, query=query)
    print_json(result)


def cmd_reservation_cancel(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    path = f"/app/commerce/api/v1/users/{user_id}/cart/reservation"
    result = request_json(session, "DELETE", path)
    print_json(result)


def cmd_account_profile(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    path = f"/app/commerce/api/v1/users/{user_id}"
    result = request_json(session, "GET", path)
    print_json(result)


def cmd_account_addresses_list(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    path = f"/app/commerce/api/v1/users/{user_id}/addresses/shipping-addresses"
    result = request_json(session, "GET", path)
    print_json(result)


def cmd_account_addresses_add(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    data = load_json_file(args.payload_file)
    if not isinstance(data, dict):
        raise ValueError("Plik payload musi zawierać obiekt JSON.")
    body = data if "shippingAddress" in data else {"shippingAddress": data}
    path = f"/app/commerce/api/v1/users/{user_id}/addresses/shipping-addresses"
    result = request_json(session, "POST", path, data=body)
    print_json(result)


def cmd_account_addresses_delete(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    path = (
        f"/app/commerce/api/v1/users/{user_id}/addresses/shipping-addresses/"
        f"{args.address_id}"
    )
    result = request_json(session, "DELETE", path)
    print_json(result)


def cmd_account_consents_update(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    body = load_json_file(args.payload_file)
    if not isinstance(body, dict):
        raise ValueError("Plik payload musi zawierać obiekt JSON.")
    path = f"/app/commerce/api/v1/users/{user_id}/consents"
    result = request_json(session, "PUT", path, data=body)
    print_json(result)


def cmd_account_rules_accept(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    if args.payload_file:
        body = load_json_file(args.payload_file)
        if not isinstance(body, dict):
            raise ValueError("Plik payload musi zawierać obiekt JSON.")
    else:
        if not args.rule_id:
            raise ValueError("Podaj --rule-id albo --payload-file.")
        body = {"acceptedRules": args.rule_id}
    path = f"/app/commerce/api/v1/users/{user_id}/rules"
    result = request_json(session, "PUT", path, data=body)
    print_json(result)


def cmd_account_vouchers(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    path = f"/app/commerce/api/v1/users/{user_id}/vouchers"
    result = request_json(session, "GET", path)
    print_json(result)


def cmd_account_membership_cards(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    path = f"/app/commerce/api/v1/users/{user_id}/membership-cards"
    result = request_json(session, "GET", path)
    print_json(result)


def cmd_account_membership_points(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    path = f"/app/commerce/api/v1/users/{user_id}/membership/points"
    query = [f"pageIndex={args.page_index}", f"pageSize={args.page_size}"]
    result = request_json(session, "GET", path, query=query)
    print_json(result)


def cmd_account_payments(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    path = f"/app/commerce/api/v1/users/{user_id}/payments"
    result = request_json(session, "GET", path)
    print_json(result)


def extract_orders_list(payload: Any) -> List[Dict[str, Any]]:
    if isinstance(payload, list):
        return [x for x in payload if isinstance(x, dict)]
    if isinstance(payload, dict):
        for key in ("items", "orders", "results", "data"):
            value = payload.get(key)
            if isinstance(value, list):
                return [x for x in value if isinstance(x, dict)]
    return []


def extract_order_datetime(order: Dict[str, Any]) -> Optional[str]:
    for key in ("createdAt", "created", "placedAt", "orderDate", "date"):
        value = order.get(key)
        if isinstance(value, str) and value:
            return value
    return None


def extract_order_total(order: Dict[str, Any]) -> Optional[float]:
    candidates: List[float] = []
    for key in ("total", "totalValue", "amount", "grossValue", "orderValue", "finalPrice"):
        value = order.get(key)
        if isinstance(value, (int, float)):
            candidates.append(float(value))
        elif isinstance(value, dict):
            total_value = value.get("_total")
            if isinstance(total_value, (int, float)):
                candidates.append(float(total_value))

    for section_key in ("pricing", "payment", "summary", "totals", "orderPricing"):
        section = order.get(section_key)
        if not isinstance(section, dict):
            continue
        for value_key in (
            "totalPayment",
            "totalWithDeliveryCostAfterVoucherPayment",
            "totalWithDeliveryCost",
            "total",
        ):
            value = section.get(value_key)
            if isinstance(value, (int, float)):
                candidates.append(float(value))
            elif isinstance(value, dict):
                total_value = value.get("_total")
                if isinstance(total_value, (int, float)):
                    candidates.append(float(total_value))

    if not candidates:
        return None
    positives = [x for x in candidates if x > 0]
    return max(positives) if positives else max(candidates)


def cmd_orders_list(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    path = f"/app/commerce/api/v1/users/{user_id}/orders"
    if args.all_pages:
        all_items: List[Dict[str, Any]] = []
        page_index = args.page_index
        while True:
            query = [f"pageIndex={page_index}", f"pageSize={args.page_size}"]
            payload = request_json(session, "GET", path, query=query)
            items = extract_orders_list(payload)
            if not items:
                break
            all_items.extend(items)
            if len(items) < args.page_size:
                break
            page_index += 1
            if page_index - args.page_index > 100:
                break
        result: Any = all_items
    else:
        query = [f"pageIndex={args.page_index}", f"pageSize={args.page_size}"]
        result = request_json(session, "GET", path, query=query)

    if args.raw:
        print_json(result)
        return

    items = extract_orders_list(result)
    compact = []
    for order in items:
        compact.append(
            {
                "id": order.get("id") or order.get("orderId"),
                "status": order.get("status") or order.get("orderStatus"),
                "createdAt": extract_order_datetime(order),
                "totalPLN": extract_order_total(order),
            }
        )

    total_values = [x["totalPLN"] for x in compact if isinstance(x.get("totalPLN"), (int, float))]
    summary = {
        "count": len(compact),
        "sumPLN": round(sum(total_values), 2) if total_values else None,
        "avgPLN": round(sum(total_values) / len(total_values), 2) if total_values else None,
    }
    print_json({"summary": summary, "orders": compact})


def cmd_orders_get(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    path = f"/app/commerce/api/v1/users/{user_id}/orders/{args.order_id}"
    result = request_json(session, "GET", path)
    print_json(result)


def cmd_orders_delivery(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    path = f"/app/commerce/api/v1/users/{user_id}/orders/{args.order_id}/delivery"
    result = request_json(session, "GET", path)
    print_json(result)


def cmd_orders_payments(args: argparse.Namespace) -> None:
    session = load_session()
    user_id = require_user_id(session, args.user_id)
    path = f"/app/commerce/api/v1/users/{user_id}/orders/{args.order_id}/payments"
    result = request_json(session, "GET", path)
    print_json(result)


def extract_refresh_token_from_curl_body(body: Optional[str]) -> Optional[str]:
    if not body:
        return None
    for k, v in parse_qsl(body, keep_blank_values=True):
        if k == "refresh_token" and v:
            return v
    return None


def extract_refresh_token_from_cookie(cookie_header: Optional[str]) -> Optional[str]:
    if not cookie_header:
        return None
    for part in cookie_header.split(";"):
        if "=" not in part:
            continue
        k, v = part.split("=", 1)
        if k.strip() == "rtokenN":
            raw = v.strip()
            if "|" in raw:
                return raw.split("|", 1)[1]
            return raw
    return None


def cmd_auth_refresh(args: argparse.Namespace) -> None:
    session = load_session()
    refresh_token = args.refresh_token or session.get("refresh_token")
    if not refresh_token:
        raise ValueError("Brak refresh tokena. Podaj --refresh-token albo wczytaj go przez session from-curl.")

    payload = {
        "grant_type": "refresh_token",
        "refresh_token": refresh_token,
    }
    result = request_json(
        session=session,
        method="POST",
        path_or_url="/app/commerce/connect/token",
        data=payload,
        data_format="form",
    )

    if isinstance(result, dict):
        access_token = result.get("access_token")
        new_refresh = result.get("refresh_token")
        if access_token:
            session["token"] = str(access_token)
            session.setdefault("headers", {})["Authorization"] = f"Bearer {access_token}"
        if new_refresh:
            session["refresh_token"] = str(new_refresh)
        save_session(session)
    print_json(result)


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="frisco-cli",
        description="CLI do obsługi endpointów Frisco znalezionych w HAR (XHR).",
    )
    sub = parser.add_subparsers(dest="command", required=True)

    # session
    p_session = sub.add_parser("session", help="Zarządzanie sesją (token, headers, user_id).")
    sp_session = p_session.add_subparsers(dest="session_cmd", required=True)

    p_from_curl = sp_session.add_parser("from-curl", help="Wczytaj sesję z komendy curl.")
    p_from_curl.add_argument("--curl", required=True, help="Cała komenda curl w cudzysłowie.")
    p_from_curl.set_defaults(func=cmd_session_from_curl)

    p_show = sp_session.add_parser("show", help="Pokaż aktualną sesję (wrażliwe dane ukryte).")
    p_show.set_defaults(func=cmd_session_show)

    # har
    p_har = sub.add_parser("har", help="Import/obsługa HAR.")
    sp_har = p_har.add_subparsers(dest="har_cmd", required=True)
    p_import = sp_har.add_parser("import", help="Importuj endpointy XHR z HAR.")
    p_import.add_argument("--path", default=DEFAULT_HAR_PATH, help="Ścieżka do pliku HAR.")
    p_import.set_defaults(func=cmd_har_import)

    # xhr
    p_xhr = sub.add_parser("xhr", help="Niskopoziomowy dostęp do endpointów XHR.")
    sp_xhr = p_xhr.add_subparsers(dest="xhr_cmd", required=True)
    p_xhr_list = sp_xhr.add_parser("list", help="Wylistuj zaimportowane endpointy XHR.")
    p_xhr_list.add_argument("--contains", help="Filtr po fragmencie ścieżki/metody.")
    p_xhr_list.set_defaults(func=cmd_xhr_list)

    p_xhr_call = sp_xhr.add_parser("call", help="Wywołaj dowolny endpoint.")
    p_xhr_call.add_argument("--method", required=True, help="HTTP method, np. GET/POST/PUT/DELETE")
    p_xhr_call.add_argument("--path-or-url", required=True, help="Ścieżka względna lub pełny URL.")
    p_xhr_call.add_argument("--query", action="append", help="Parametr query key=value (powtarzalny).")
    p_xhr_call.add_argument("--header", action="append", help="Nagłówek Key: Value (powtarzalny).")
    p_xhr_call.add_argument("--data", help="JSON body albo key=value&k2=v2")
    p_xhr_call.add_argument(
        "--data-format",
        default="auto",
        choices=["auto", "json", "form", "raw"],
        help="Format body: auto/json/form/raw",
    )
    p_xhr_call.set_defaults(func=cmd_xhr_call)

    # products
    p_products = sub.add_parser("products", help="Operacje produktowe.")
    sp_products = p_products.add_subparsers(dest="products_cmd", required=True)

    p_search = sp_products.add_parser("search", help="Szukaj produktów.")
    p_search.add_argument("--search", required=True, help="Fraza wyszukiwania.")
    p_search.add_argument("--page-index", default=1, type=int)
    p_search.add_argument("--page-size", default=84, type=int)
    p_search.add_argument("--delivery-method", default="Van")
    p_search.add_argument("--user-id")
    p_search.set_defaults(func=cmd_products_search)

    p_by_ids = sp_products.add_parser("by-ids", help="Pobierz produkty po productIds.")
    p_by_ids.add_argument("--product-id", action="append", required=True)
    p_by_ids.add_argument("--user-id")
    p_by_ids.set_defaults(func=cmd_products_by_ids)

    # cart
    p_cart = sub.add_parser("cart", help="Operacje na koszyku.")
    sp_cart = p_cart.add_subparsers(dest="cart_cmd", required=True)

    p_cart_show = sp_cart.add_parser("show", help="Pobierz koszyk.")
    p_cart_show.add_argument("--user-id")
    p_cart_show.set_defaults(func=cmd_cart_show)

    p_cart_add = sp_cart.add_parser("add", help="Dodaj/ustaw ilość produktu w koszyku.")
    p_cart_add.add_argument("--product-id", required=True)
    p_cart_add.add_argument("--quantity", default=1, type=int)
    p_cart_add.add_argument("--user-id")
    p_cart_add.set_defaults(func=cmd_cart_add)

    p_cart_remove = sp_cart.add_parser("remove", help="Usuń produkt z koszyka (quantity=0).")
    p_cart_remove.add_argument("--product-id", required=True)
    p_cart_remove.add_argument("--user-id")
    p_cart_remove.set_defaults(func=cmd_cart_remove)

    # reservation
    p_res = sub.add_parser("reservation", help="Planowanie i status rezerwacji.")
    sp_res = p_res.add_subparsers(dest="reservation_cmd", required=True)

    p_delivery = sp_res.add_parser("delivery-options", help="Opcje dostawy/płatności po kodzie pocztowym.")
    p_delivery.add_argument("--postcode", required=True)
    p_delivery.set_defaults(func=cmd_reservation_delivery_options)

    p_calendar = sp_res.add_parser("calendar", help="Dostępne okna czasowe dla adresu.")
    p_calendar.add_argument("--shipping-address-file", required=True, help="JSON z shippingAddress.")
    p_calendar.add_argument("--date", help="Opcjonalnie data YYYY-M-D")
    p_calendar.add_argument("--user-id")
    p_calendar.set_defaults(func=cmd_reservation_calendar)

    p_slots = sp_res.add_parser(
        "slots",
        help="Pobierz dostępne godziny dostawy dla kolejnych dni (w tym dzisiaj).",
    )
    p_slots.add_argument("--days", type=int, default=3, help="Ile kolejnych dni sprawdzić.")
    p_slots.add_argument("--start-date", help="Data startowa YYYY-MM-DD (domyślnie dziś).")
    p_slots.add_argument("--shipping-address-file", help="Opcjonalny JSON z adresem.")
    p_slots.add_argument("--user-id")
    p_slots.add_argument("--raw", action="store_true", help="Zwróć surową odpowiedź API.")
    p_slots.set_defaults(func=cmd_reservation_slots)

    p_reserve = sp_res.add_parser(
        "reserve",
        help="Zarezerwuj slot po dacie i godzinach (np. 06:00-07:00).",
    )
    p_reserve.add_argument("--date", required=True, help="Data YYYY-MM-DD")
    p_reserve.add_argument("--from-time", required=True, help="Godzina startu HH:MM")
    p_reserve.add_argument("--to-time", required=True, help="Godzina końca HH:MM")
    p_reserve.add_argument("--shipping-address-file", help="Opcjonalny JSON z adresem.")
    p_reserve.add_argument("--user-id")
    p_reserve.set_defaults(func=cmd_reservation_reserve)

    p_plan = sp_res.add_parser("plan", help="Zaplanuj rezerwację koszyka z payloadu JSON.")
    p_plan.add_argument("--payload-file", required=True, help="JSON jak w /cart/reservation")
    p_plan.add_argument("--user-id")
    p_plan.set_defaults(func=cmd_reservation_plan)

    p_status = sp_res.add_parser("status", help="Status zamówień/rezerwacji użytkownika.")
    p_status.add_argument("--page-index", default=1, type=int)
    p_status.add_argument("--page-size", default=20, type=int)
    p_status.add_argument("--user-id")
    p_status.set_defaults(func=cmd_reservation_status)

    p_cancel = sp_res.add_parser("cancel", help="Anuluj aktywną rezerwację koszyka.")
    p_cancel.add_argument("--user-id")
    p_cancel.set_defaults(func=cmd_reservation_cancel)

    # account
    p_account = sub.add_parser("account", help="Operacje zarządzania kontem.")
    sp_account = p_account.add_subparsers(dest="account_cmd", required=True)

    p_profile = sp_account.add_parser("profile", help="Pobierz profil użytkownika.")
    p_profile.add_argument("--user-id")
    p_profile.set_defaults(func=cmd_account_profile)

    p_addresses = sp_account.add_parser("addresses", help="Adresy dostawy.")
    sp_addresses = p_addresses.add_subparsers(dest="addresses_cmd", required=True)

    p_addr_list = sp_addresses.add_parser("list", help="Lista adresów.")
    p_addr_list.add_argument("--user-id")
    p_addr_list.set_defaults(func=cmd_account_addresses_list)

    p_addr_add = sp_addresses.add_parser("add", help="Dodaj adres (JSON).")
    p_addr_add.add_argument("--payload-file", required=True, help="JSON address lub {shippingAddress:{...}}")
    p_addr_add.add_argument("--user-id")
    p_addr_add.set_defaults(func=cmd_account_addresses_add)

    p_addr_del = sp_addresses.add_parser("delete", help="Usuń adres po UUID.")
    p_addr_del.add_argument("--address-id", required=True)
    p_addr_del.add_argument("--user-id")
    p_addr_del.set_defaults(func=cmd_account_addresses_delete)

    p_consents = sp_account.add_parser("consents", help="Zarządzanie zgodami.")
    sp_consents = p_consents.add_subparsers(dest="consents_cmd", required=True)
    p_consents_update = sp_consents.add_parser("update", help="Aktualizuj zgody payloadem JSON.")
    p_consents_update.add_argument("--payload-file", required=True)
    p_consents_update.add_argument("--user-id")
    p_consents_update.set_defaults(func=cmd_account_consents_update)

    p_rules = sp_account.add_parser("rules", help="Akceptacja regulaminów.")
    sp_rules = p_rules.add_subparsers(dest="rules_cmd", required=True)
    p_rules_accept = sp_rules.add_parser("accept", help="Akceptuj reguły.")
    p_rules_accept.add_argument("--rule-id", action="append", help="Powtarzalne UUID reguł do akceptacji.")
    p_rules_accept.add_argument("--payload-file", help="Alternatywnie pełny payload JSON.")
    p_rules_accept.add_argument("--user-id")
    p_rules_accept.set_defaults(func=cmd_account_rules_accept)

    p_vouchers = sp_account.add_parser("vouchers", help="Pobierz vouchery.")
    p_vouchers.add_argument("--user-id")
    p_vouchers.set_defaults(func=cmd_account_vouchers)

    p_payments = sp_account.add_parser("payments", help="Pobierz metody płatności.")
    p_payments.add_argument("--user-id")
    p_payments.set_defaults(func=cmd_account_payments)

    p_membership = sp_account.add_parser("membership", help="Membership cards/points.")
    sp_membership = p_membership.add_subparsers(dest="membership_cmd", required=True)

    p_cards = sp_membership.add_parser("cards", help="Pobierz membership cards.")
    p_cards.add_argument("--user-id")
    p_cards.set_defaults(func=cmd_account_membership_cards)

    p_points = sp_membership.add_parser("points", help="Pobierz historię punktów.")
    p_points.add_argument("--page-index", default=1, type=int)
    p_points.add_argument("--page-size", default=25, type=int)
    p_points.add_argument("--user-id")
    p_points.set_defaults(func=cmd_account_membership_points)

    # orders
    p_orders = sub.add_parser("orders", help="Szczegóły zamówień.")
    sp_orders = p_orders.add_subparsers(dest="orders_cmd", required=True)

    p_orders_list = sp_orders.add_parser("list", help="Lista zamówień.")
    p_orders_list.add_argument("--page-index", default=1, type=int)
    p_orders_list.add_argument("--page-size", default=10, type=int)
    p_orders_list.add_argument("--all-pages", action="store_true", help="Pobierz wszystkie strony.")
    p_orders_list.add_argument("--raw", action="store_true", help="Zwróć surową odpowiedź API.")
    p_orders_list.add_argument("--user-id")
    p_orders_list.set_defaults(func=cmd_orders_list)

    p_orders_get = sp_orders.add_parser("get", help="Szczegóły jednego zamówienia.")
    p_orders_get.add_argument("--order-id", required=True)
    p_orders_get.add_argument("--user-id")
    p_orders_get.set_defaults(func=cmd_orders_get)

    p_orders_delivery = sp_orders.add_parser("delivery", help="Dostawa dla zamówienia.")
    p_orders_delivery.add_argument("--order-id", required=True)
    p_orders_delivery.add_argument("--user-id")
    p_orders_delivery.set_defaults(func=cmd_orders_delivery)

    p_orders_payments = sp_orders.add_parser("payments", help="Płatności dla zamówienia.")
    p_orders_payments.add_argument("--order-id", required=True)
    p_orders_payments.add_argument("--user-id")
    p_orders_payments.set_defaults(func=cmd_orders_payments)

    # auth
    p_auth = sub.add_parser("auth", help="Autoryzacja i odświeżanie tokena.")
    sp_auth = p_auth.add_subparsers(dest="auth_cmd", required=True)

    p_refresh = sp_auth.add_parser("refresh-token", help="Odśwież access token przez refresh token.")
    p_refresh.add_argument("--refresh-token", help="Opcjonalny refresh token (inaczej z sesji).")
    p_refresh.set_defaults(func=cmd_auth_refresh)

    return parser


def main() -> int:
    parser = build_parser()
    args = parser.parse_args()
    try:
        args.func(args)
        return 0
    except Exception as e:
        print(f"BŁĄD: {e}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
