"""Tool definitions for the LLM (OpenAI-compatible format)."""

TOOLS = [
    {
        "type": "function",
        "function": {
            "name": "place_trade",
            "description": (
                "Abre una nueva posición de futuros perpetuos en OKX. "
                "Usar solo cuando TODAS las condiciones de entrada de la estrategia se cumplen."
            ),
            "parameters": {
                "type": "object",
                "properties": {
                    "symbol": {
                        "type": "string",
                        "description": "Par a operar. Ejemplo: BTC-USDT-SWAP",
                        "enum": [
                            "BTC-USDT-SWAP", "ETH-USDT-SWAP", "BNB-USDT-SWAP", "SOL-USDT-SWAP",
                            "XRP-USDT-SWAP", "DOGE-USDT-SWAP", "ADA-USDT-SWAP",
                            "AVAX-USDT-SWAP", "TRX-USDT-SWAP", "TON-USDT-SWAP",
                        ],
                    },
                    "direction": {
                        "type": "string",
                        "enum": ["long", "short"],
                        "description": "Dirección de la posición.",
                    },
                    "size_usdt": {
                        "type": "number",
                        "description": "Margen en USDT a usar. Debe ser el 20% del balance disponible.",
                    },
                    "sl_price": {
                        "type": "number",
                        "description": "Precio exacto del Stop Loss.",
                    },
                    "tp_price": {
                        "type": "number",
                        "description": "Precio exacto del Take Profit.",
                    },
                },
                "required": ["symbol", "direction", "size_usdt", "sl_price", "tp_price"],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "close_trade",
            "description": (
                "Cierra una posición abierta existente. "
                "Usar cuando las condiciones de salida anticipada se cumplen."
            ),
            "parameters": {
                "type": "object",
                "properties": {
                    "symbol": {
                        "type": "string",
                        "description": "Par cuya posición cerrar. Ejemplo: BTC-USDT-SWAP",
                    },
                    "reason": {
                        "type": "string",
                        "description": "Motivo del cierre. Ser específico: qué condición se activó.",
                    },
                },
                "required": ["symbol", "reason"],
            },
        },
    },
    {
        "type": "function",
        "function": {
            "name": "no_action",
            "description": (
                "No tomar ninguna acción este ciclo. "
                "Usar cuando ninguna condición de entrada o salida se cumple."
            ),
            "parameters": {
                "type": "object",
                "properties": {
                    "reason": {
                        "type": "string",
                        "description": "Explicación de por qué no se toma acción. Mencionar el par y qué condición falta.",
                    },
                },
                "required": ["reason"],
            },
        },
    },
]
