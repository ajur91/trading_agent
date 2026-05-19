import sqlite3
import json
import os
from datetime import datetime

DB_PATH = os.environ.get("DB_PATH", "/app/data/agent.db")


def get_conn():
    conn = sqlite3.connect(DB_PATH)
    conn.row_factory = sqlite3.Row
    return conn


def init_db():
    with get_conn() as conn:
        conn.executescript("""
        CREATE TABLE IF NOT EXISTS decisions (
            id          INTEGER PRIMARY KEY AUTOINCREMENT,
            timestamp   TEXT    NOT NULL,
            action      TEXT    NOT NULL,
            symbol      TEXT,
            params      TEXT,
            reasoning   TEXT,
            executed    INTEGER DEFAULT 0,
            error       TEXT
        );

        CREATE TABLE IF NOT EXISTS trades (
            id            INTEGER PRIMARY KEY AUTOINCREMENT,
            symbol        TEXT    NOT NULL,
            direction     TEXT    NOT NULL,
            size_usdt     REAL,
            entry_price   REAL,
            sl_price      REAL,
            tp_price      REAL,
            contracts     REAL,
            order_id      TEXT,
            open_time     TEXT    NOT NULL,
            close_time    TEXT,
            close_price   REAL,
            pnl_usdt      REAL,
            close_reason  TEXT,
            status        TEXT    DEFAULT 'open'
        );
        """)


def log_decision(action: str, symbol: str | None, params: dict | None,
                 reasoning: str, executed: bool, error: str | None = None):
    with get_conn() as conn:
        conn.execute(
            """INSERT INTO decisions (timestamp, action, symbol, params, reasoning, executed, error)
               VALUES (?, ?, ?, ?, ?, ?, ?)""",
            (
                datetime.utcnow().isoformat(),
                action,
                symbol,
                json.dumps(params) if params else None,
                reasoning,
                1 if executed else 0,
                error,
            ),
        )


def open_trade(symbol: str, direction: str, size_usdt: float, entry_price: float,
               sl_price: float, tp_price: float, contracts: float, order_id: str):
    with get_conn() as conn:
        conn.execute(
            """INSERT INTO trades
               (symbol, direction, size_usdt, entry_price, sl_price, tp_price, contracts, order_id, open_time)
               VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)""",
            (symbol, direction, size_usdt, entry_price, sl_price, tp_price,
             contracts, order_id, datetime.utcnow().isoformat()),
        )


def close_trade(symbol: str, close_price: float, pnl_usdt: float, reason: str):
    with get_conn() as conn:
        conn.execute(
            """UPDATE trades SET close_time=?, close_price=?, pnl_usdt=?, close_reason=?, status='closed'
               WHERE symbol=? AND status='open'""",
            (datetime.utcnow().isoformat(), close_price, pnl_usdt, reason, symbol),
        )


def get_open_trades() -> list[dict]:
    with get_conn() as conn:
        rows = conn.execute("SELECT * FROM trades WHERE status='open'").fetchall()
        return [dict(r) for r in rows]
