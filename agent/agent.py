"""Core agent logic: build context, call LLM, dispatch tool."""
import json
import logging
import os
from pathlib import Path

from openai import OpenAI
from openai.types.chat import ChatCompletionMessageToolCall

import db
import executor
from tools import TOOLS

log = logging.getLogger(__name__)

OLLAMA_BASE_URL = os.environ.get("OLLAMA_BASE_URL", "http://ollama:11434/v1")
LLM_MODEL = os.environ.get("LLM_MODEL", "qwen2.5:14b")
DRY_RUN = os.environ.get("DRY_RUN", "false").lower() == "true"

_client: OpenAI | None = None


def _llm() -> OpenAI:
    global _client
    if _client is None:
        _client = OpenAI(base_url=OLLAMA_BASE_URL, api_key="ollama")
    return _client


def _load_prompt() -> str:
    base = Path("/app/prompts/system_base.md").read_text()
    strategy = Path("/app/prompts/strategy.md").read_text()
    return f"{base}\n\n---\n\n{strategy}"


def _build_user_message(indicators: dict[str, dict], positions: list[dict], balance: dict) -> str:
    lines = [
        "## Estado de la cuenta",
        f"- Balance disponible: {balance.get('available', 0):.2f} USDT",
        f"- Balance total: {balance.get('total', 0):.2f} USDT",
        f"- Posiciones abiertas: {len(positions)}",
        "",
    ]

    if positions:
        lines.append("## Posiciones actuales")
        for p in positions:
            lines.append(
                f"- {p['symbol']} | {p['side'].upper()} | "
                f"entrada: {p['entry_price']:.4f} | marca: {p['mark_price']:.4f} | "
                f"PnL no realizado: {p['unrealized_pnl']:.2f} USDT"
            )
        lines.append("")

    lines.append("## Datos de mercado actuales")

    for symbol, ind in indicators.items():
        lines.append(f"\n### {symbol}")
        if "error" in ind:
            lines.append(f"- ERROR obteniendo datos: {ind['error']}")
            continue
        lines.append(f"- Precio actual: {ind.get('price', 0):.4f} USDT")
        lines.append(f"- EMA9 (4H): {ind.get('ema9_4h', 0):.4f}")
        lines.append(f"- EMA21 (4H): {ind.get('ema21_4h', 0):.4f}")
        lines.append(f"- EMA50 (1D): {ind.get('ema50_1d', 0):.4f}")
        lines.append(f"- ADX14 (4H): {ind.get('adx14_4h', 0):.2f}")
        lines.append(f"- RSI14 (4H): {ind.get('rsi14_4h', 0):.2f}")
        lines.append(f"- ATR14 (4H): {ind.get('atr14_4h', 0):.4f}")
        lines.append(f"- Cruce EMA9/21 en 4H: {ind.get('ema_cross', 'none')} (hace {ind.get('ema_cross_ago', -1)} velas)")
        lines.append(f"- Tendencia vs EMA50 diaria: {ind.get('trend_vs_ema50', 'neutral').upper()}")

    return "\n".join(lines)


def run_cycle():
    log.info("=== Iniciando ciclo de análisis ===")

    # Gather market context
    try:
        indicators = executor.get_all_indicators()
        positions = executor.get_positions()
        balance = executor.get_balance()
    except Exception as e:
        log.error(f"Error obteniendo datos de mercado: {e}")
        db.log_decision("error", None, None, f"Market data fetch failed: {e}", False, str(e))
        return

    user_msg = _build_user_message(indicators, positions, balance)
    system_prompt = _load_prompt()

    log.debug("Mensaje al LLM:\n%s", user_msg)

    # Call LLM
    try:
        response = _llm().chat.completions.create(
            model=LLM_MODEL,
            messages=[
                {"role": "system", "content": system_prompt},
                {"role": "user", "content": user_msg},
            ],
            tools=TOOLS,
            tool_choice="required",
            temperature=0.1,
        )
    except Exception as e:
        log.error(f"LLM call failed: {e}")
        db.log_decision("error", None, None, f"LLM call failed: {e}", False, str(e))
        return

    msg = response.choices[0].message
    reasoning = msg.content or ""

    if not msg.tool_calls:
        log.warning("LLM returned no tool call — logging as no_action")
        db.log_decision("no_action", None, None, reasoning or "No tool call returned by LLM", False,
                        "LLM did not return a tool call")
        return

    tool_call: ChatCompletionMessageToolCall = msg.tool_calls[0]
    action = tool_call.function.name

    try:
        params = json.loads(tool_call.function.arguments)
    except json.JSONDecodeError as e:
        log.error(f"Failed to parse tool arguments: {e}")
        db.log_decision(action, None, None, reasoning, False, f"JSON parse error: {e}")
        return

    log.info(f"LLM decision: {action} | params: {params}")
    log.info(f"Reasoning: {reasoning[:300]}...")

    _dispatch(action, params, reasoning, balance)


def _dispatch(action: str, params: dict, reasoning: str, balance: dict):
    if action == "no_action":
        reason = params.get("reason", "")
        log.info(f"No action: {reason}")
        db.log_decision("no_action", None, params, reasoning, True)
        return

    if action == "close_trade":
        symbol = params.get("symbol", "")
        reason = params.get("reason", "")

        if DRY_RUN:
            log.info(f"[DRY RUN] Would close {symbol}: {reason}")
            db.log_decision("close_trade", symbol, params, reasoning, False, "DRY_RUN mode")
            return

        try:
            result = executor.close_trade(symbol, reason)
            log.info(f"Closed position {symbol}: {result}")
            db.log_decision("close_trade", symbol, params, reasoning, True)
            db.close_trade(symbol, 0.0, 0.0, reason)
        except Exception as e:
            log.error(f"close_trade failed for {symbol}: {e}")
            db.log_decision("close_trade", symbol, params, reasoning, False, str(e))
        return

    if action == "place_trade":
        symbol = params.get("symbol", "")
        direction = params.get("direction", "")
        sl_price = float(params.get("sl_price", 0))
        tp_price = float(params.get("tp_price", 0))

        # Calculate size_usdt as 20% of available balance
        available = balance.get("available", 0)
        size_usdt = round(available * 0.20, 2)
        if size_usdt < 5:
            log.warning(f"Insufficient balance for trade: {available:.2f} USDT available")
            db.log_decision("place_trade", symbol, params, reasoning, False,
                            f"Insufficient balance: {available:.2f} USDT")
            return

        if DRY_RUN:
            log.info(f"[DRY RUN] Would open {direction.upper()} {symbol} | "
                     f"size={size_usdt} USDT | SL={sl_price} | TP={tp_price}")
            db.log_decision("place_trade", symbol, {**params, "size_usdt": size_usdt},
                            reasoning, False, "DRY_RUN mode")
            return

        try:
            result = executor.place_trade(symbol, direction, size_usdt, sl_price, tp_price)
            log.info(f"Opened {direction} {symbol}: {result}")
            db.log_decision("place_trade", symbol, {**params, "size_usdt": size_usdt}, reasoning, True)
            db.open_trade(
                symbol=symbol,
                direction=direction,
                size_usdt=size_usdt,
                entry_price=result.get("entry_price", 0),
                sl_price=sl_price,
                tp_price=tp_price,
                contracts=result.get("contracts", 0),
                order_id=result.get("order_id", ""),
            )
        except Exception as e:
            log.error(f"place_trade failed for {symbol}: {e}")
            db.log_decision("place_trade", symbol, {**params, "size_usdt": size_usdt},
                            reasoning, False, str(e))
