"""HTTP client for the market-api Go service."""
import httpx
import os
from typing import Any

MARKET_API_URL = os.environ.get("MARKET_API_URL", "http://market-api:8080")
TIMEOUT = 30.0

SYMBOLS = [s.strip() for s in os.environ.get(
    "SYMBOLS", "BTC-USDT-SWAP,ETH-USDT-SWAP,SOL-USDT-SWAP,BNB-USDT-SWAP"
).split(",")]


def _get(path: str, params: dict | None = None) -> Any:
    with httpx.Client(timeout=TIMEOUT) as client:
        resp = client.get(f"{MARKET_API_URL}{path}", params=params)
        resp.raise_for_status()
        return resp.json()


def _post(path: str, body: dict) -> Any:
    with httpx.Client(timeout=TIMEOUT) as client:
        resp = client.post(f"{MARKET_API_URL}{path}", json=body)
        resp.raise_for_status()
        return resp.json()


def get_indicators(symbol: str) -> dict:
    return _get("/v1/market/indicators", {"symbol": symbol})


def get_all_indicators() -> dict[str, dict]:
    results = {}
    for symbol in SYMBOLS:
        try:
            results[symbol] = get_indicators(symbol)
        except Exception as e:
            results[symbol] = {"error": str(e), "symbol": symbol}
    return results


def get_balance() -> dict:
    return _get("/v1/account/balance")


def get_positions() -> list[dict]:
    return _get("/v1/account/positions")


def place_trade(symbol: str, direction: str, size_usdt: float,
                sl_price: float, tp_price: float) -> dict:
    return _post("/v1/orders/place", {
        "symbol": symbol,
        "direction": direction,
        "size_usdt": size_usdt,
        "sl_price": sl_price,
        "tp_price": tp_price,
    })


def close_trade(symbol: str, reason: str) -> dict:
    return _post("/v1/orders/close", {
        "symbol": symbol,
        "reason": reason,
    })


def health_check() -> bool:
    try:
        _get("/health")
        return True
    except Exception:
        return False
