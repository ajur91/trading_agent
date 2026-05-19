# Estrategia: HTF Trend v1

## Universo de pares autorizados
- BTC-USDT-SWAP
- ETH-USDT-SWAP
- SOL-USDT-SWAP
- BNB-USDT-SWAP

## Parámetros de riesgo
- **Apalancamiento:** 3× (configurado en el servidor, no ajustable aquí)
- **Tamaño por trade:** 20% del balance disponible en USDT
- **Máximo de posiciones simultáneas:** 2
- **Stop Loss:** 1.5 × ATR(14) desde el precio de entrada
- **Take Profit:** mínimo 3:1 RR (3 × distancia al Stop Loss)

## Señal de entrada LONG

**Todos** los siguientes deben cumplirse simultáneamente:

1. `trend_vs_ema50 == "bullish"` — el precio está POR ENCIMA de la EMA50 diaria
2. `ema_cross == "bullish"` Y `ema_cross_ago <= 2` — hubo un cruce alcista de EMA9 sobre EMA21 en 4H hace máximo 2 velas
3. `adx14_4h > 25` — hay tendencia definida en 4H
4. `rsi14_4h >= 45 AND rsi14_4h <= 72` — RSI en zona de momentum alcista, sin sobrecompra extrema
5. No existe posición abierta en ese par

**Cálculo de SL y TP para LONG:**
- `sl_price = precio_entrada - (1.5 × atr14_4h)`
- `tp_price = precio_entrada + (3 × distancia_al_SL)`
- Es decir: `tp_price = precio_entrada + (4.5 × atr14_4h)`

## Señal de entrada SHORT

**Todos** los siguientes deben cumplirse simultáneamente:

1. `trend_vs_ema50 == "bearish"` — el precio está POR DEBAJO de la EMA50 diaria
2. `ema_cross == "bearish"` Y `ema_cross_ago <= 2` — hubo un cruce bajista de EMA21 sobre EMA9 en 4H hace máximo 2 velas
3. `adx14_4h > 25` — hay tendencia definida en 4H
4. `rsi14_4h >= 28 AND rsi14_4h <= 55` — RSI en zona de momentum bajista, sin sobreventa extrema
5. No existe posición abierta en ese par

**Cálculo de SL y TP para SHORT:**
- `sl_price = precio_entrada + (1.5 × atr14_4h)`
- `tp_price = precio_entrada - (3 × distancia_al_SL)`
- Es decir: `tp_price = precio_entrada - (4.5 × atr14_4h)`

## Condiciones de cierre anticipado

Considera usar `close_trade` si se cumple cualquiera de estas condiciones en una posición abierta:

- La tendencia `trend_vs_ema50` se invirtió respecto al momento de entrada
- `adx14_4h < 20` — la tendencia perdió fuerza significativamente
- El EMA cross se invirtió (ej: tenías una posición LONG abierta desde un cruce bullish, y ahora hay un cruce bearish)

## Cuándo definitivamente NO operar (usar `no_action`)

- `adx14_4h <= 25`: mercado sin tendencia suficiente
- RSI fuera de los rangos especificados
- `ema_cross == "none"`: no hubo cruce reciente
- `ema_cross_ago > 2`: el cruce ocurrió hace más de 2 velas (señal obsoleta)
- `trend_vs_ema50 == "neutral"`: datos insuficientes
- Ya hay 2 posiciones abiertas y ninguna viola condiciones de salida
- Datos de indicadores en 0 o inconsistentes

## Notas de gestión
- El servidor ejecuta el SL y TP como órdenes algorítmicas en OKX automáticamente.
- No necesitas monitorear el SL/TP vela a vela, pero sí revisar las condiciones de cierre anticipado en cada ciclo.
- Si tienes dudas sobre si una condición se cumple marginalmente, elige `no_action`.
