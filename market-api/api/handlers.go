package api

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"strconv"

	"github.com/beto/trading-agent/market-api/internal/indicators"
	"github.com/beto/trading-agent/market-api/internal/okx"
	"github.com/shopspring/decimal"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, map[string]string{"status": "ok"})
}

func (s *Server) handleCandles(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	bar := r.URL.Query().Get("bar")
	limitStr := r.URL.Query().Get("limit")

	if symbol == "" || bar == "" {
		writeError(w, 400, "symbol and bar are required")
		return
	}

	limit := 100
	if limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil && v > 0 {
			limit = v
		}
	}

	ctx, cancel := s.requestCtx(r)
	defer cancel()

	candles, err := s.okx.GetCandles(ctx, symbol, bar, limit)
	if err != nil {
		s.log.WithError(err).Error("GetCandles failed")
		writeError(w, 502, err.Error())
		return
	}

	type candleResp struct {
		Timestamp int64   `json:"ts"`
		Open      float64 `json:"open"`
		High      float64 `json:"high"`
		Low       float64 `json:"low"`
		Close     float64 `json:"close"`
		Volume    float64 `json:"volume"`
	}

	out := make([]candleResp, len(candles))
	for i, c := range candles {
		out[i] = candleResp{
			Timestamp: c.Timestamp.UnixMilli(),
			Open:      toFloat(c.Open),
			High:      toFloat(c.High),
			Low:       toFloat(c.Low),
			Close:     toFloat(c.Close),
			Volume:    toFloat(c.Volume),
		}
	}
	writeJSON(w, 200, out)
}

func (s *Server) handleIndicators(w http.ResponseWriter, r *http.Request) {
	symbol := r.URL.Query().Get("symbol")
	if symbol == "" {
		writeError(w, 400, "symbol is required")
		return
	}

	ctx, cancel := s.requestCtx(r)
	defer cancel()

	candles4H, err := s.okx.GetCandles(ctx, symbol, "4H", 200)
	if err != nil {
		writeError(w, 502, fmt.Sprintf("candles 4H: %s", err))
		return
	}

	candles1D, err := s.okx.GetCandles(ctx, symbol, "1D", 100)
	if err != nil {
		writeError(w, 502, fmt.Sprintf("candles 1D: %s", err))
		return
	}

	ticker, err := s.okx.GetTicker(ctx, symbol)
	if err != nil {
		writeError(w, 502, fmt.Sprintf("ticker: %s", err))
		return
	}

	ema9 := indicators.EMA(candles4H, 9)
	ema21 := indicators.EMA(candles4H, 21)
	ema50d := indicators.EMA(candles1D, 50)
	rsi14 := indicators.RSI(candles4H, 14)
	atr14 := indicators.ATR(candles4H, 14)
	adx14 := indicators.ADX(candles4H, 14)

	n4h := len(candles4H)
	n1d := len(candles1D)

	crossDir, crossAgo := indicators.CrossDirection(ema9, ema21, 3)

	trend := "neutral"
	if n1d > 0 && n4h > 0 && !ema50d[n1d-1].IsZero() {
		if ticker.Last.GreaterThan(ema50d[n1d-1]) {
			trend = "bullish"
		} else {
			trend = "bearish"
		}
	}

	writeJSON(w, 200, map[string]interface{}{
		"symbol":           symbol,
		"price":            toFloat(ticker.Last),
		"ema9_4h":          toFloat(last(ema9)),
		"ema21_4h":         toFloat(last(ema21)),
		"ema50_1d":         toFloat(last(ema50d)),
		"rsi14_4h":         toFloat(last(rsi14)),
		"atr14_4h":         toFloat(last(atr14)),
		"adx14_4h":         toFloat(last(adx14)),
		"ema_cross":        crossDir,
		"ema_cross_ago":    crossAgo,
		"trend_vs_ema50":   trend,
		"candles_4h_count": n4h,
		"candles_1d_count": n1d,
	})
}

func (s *Server) handleBalance(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.requestCtx(r)
	defer cancel()

	bal, err := s.okx.GetBalance(ctx)
	if err != nil {
		writeError(w, 502, err.Error())
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"currency":  bal.Currency,
		"total":     toFloat(bal.Total),
		"available": toFloat(bal.Available),
		"frozen":    toFloat(bal.Frozen),
	})
}

func (s *Server) handlePositions(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := s.requestCtx(r)
	defer cancel()

	positions, err := s.okx.GetPositions(ctx)
	if err != nil {
		writeError(w, 502, err.Error())
		return
	}

	type posResp struct {
		Symbol        string  `json:"symbol"`
		Side          string  `json:"side"`
		Size          float64 `json:"size"`
		EntryPrice    float64 `json:"entry_price"`
		MarkPrice     float64 `json:"mark_price"`
		UnrealizedPnL float64 `json:"unrealized_pnl"`
		Leverage      int     `json:"leverage"`
	}

	out := make([]posResp, len(positions))
	for i, p := range positions {
		out[i] = posResp{
			Symbol:        p.Symbol,
			Side:          string(p.Side),
			Size:          toFloat(p.Size),
			EntryPrice:    toFloat(p.EntryPrice),
			MarkPrice:     toFloat(p.MarkPrice),
			UnrealizedPnL: toFloat(p.UnrealizedPnL),
			Leverage:      p.Leverage,
		}
	}
	writeJSON(w, 200, out)
}

type placeOrderReq struct {
	Symbol    string  `json:"symbol"`
	Direction string  `json:"direction"` // "long" or "short"
	SizeUSDT  float64 `json:"size_usdt"`
	SLPrice   float64 `json:"sl_price"`
	TPPrice   float64 `json:"tp_price"`
}

func (s *Server) handlePlaceOrder(w http.ResponseWriter, r *http.Request) {
	var req placeOrderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request body")
		return
	}

	if req.Symbol == "" || req.Direction == "" || req.SizeUSDT <= 0 || req.SLPrice <= 0 || req.TPPrice <= 0 {
		writeError(w, 400, "symbol, direction, size_usdt, sl_price, tp_price are required and must be > 0")
		return
	}
	if req.Direction != "long" && req.Direction != "short" {
		writeError(w, 400, "direction must be 'long' or 'short'")
		return
	}

	ctx, cancel := s.requestCtx(r)
	defer cancel()

	// Get current price and instrument spec for contract size calculation
	ticker, err := s.okx.GetTicker(ctx, req.Symbol)
	if err != nil {
		writeError(w, 502, fmt.Sprintf("ticker: %s", err))
		return
	}

	instrument, err := s.okx.GetInstrument(ctx, req.Symbol)
	if err != nil {
		writeError(w, 502, fmt.Sprintf("instrument: %s", err))
		return
	}

	// Validate direction-aware risk parameters
	price := toFloat(ticker.Last)

	if req.Direction == "long" {
		if req.SLPrice >= price {
			writeError(w, 400, fmt.Sprintf("LONG sl_price (%.4f) must be below current price (%.4f)", req.SLPrice, price))
			return
		}
		if req.TPPrice <= price {
			writeError(w, 400, fmt.Sprintf("LONG tp_price (%.4f) must be above current price (%.4f)", req.TPPrice, price))
			return
		}
	} else {
		if req.SLPrice <= price {
			writeError(w, 400, fmt.Sprintf("SHORT sl_price (%.4f) must be above current price (%.4f)", req.SLPrice, price))
			return
		}
		if req.TPPrice >= price {
			writeError(w, 400, fmt.Sprintf("SHORT tp_price (%.4f) must be below current price (%.4f)", req.TPPrice, price))
			return
		}
	}

	slDist := math.Abs(price-req.SLPrice) / price
	if slDist < 0.003 {
		writeError(w, 400, "sl_price too close to current price (min 0.3% distance)")
		return
	}

	tpDist := math.Abs(req.TPPrice-price) / price
	if tpDist < slDist*2 {
		writeError(w, 400, fmt.Sprintf("tp_price does not achieve minimum 2:1 RR (sl_dist=%.2f%%, tp_dist=%.2f%%)", slDist*100, tpDist*100))
		return
	}

	// Calculate contract size:
	// notional = size_usdt * leverage
	// contracts = notional / (price * ctVal), rounded down to lotSz
	ctVal := toFloat(instrument.ContractValue)
	lotSz := toFloat(instrument.LotSize)
	if ctVal <= 0 || lotSz <= 0 {
		writeError(w, 500, "invalid instrument spec")
		return
	}

	notional := req.SizeUSDT * float64(s.leverage)
	contracts := math.Floor(notional/(price*ctVal)/lotSz) * lotSz

	minSz := toFloat(instrument.MinSize)
	if contracts < minSz {
		writeError(w, 400, fmt.Sprintf("calculated contracts (%.4f) below minimum size (%.4f). Increase size_usdt.", contracts, minSz))
		return
	}

	posSide := okx.PositionLong
	side := okx.SideBuy
	if req.Direction == "short" {
		posSide = okx.PositionShort
		side = okx.SideSell
	}

	// Set leverage
	if err := s.okx.SetLeverage(ctx, req.Symbol, s.leverage, "isolated"); err != nil {
		s.log.WithError(err).Warn("SetLeverage failed (non-fatal)")
	}

	contractsDec := decimal.NewFromFloat(contracts)

	// Place market entry order
	order, err := s.okx.PlaceMarketOrder(ctx, &okx.OrderRequest{
		Symbol:       req.Symbol,
		Side:         side,
		PositionSide: posSide,
		Size:         contractsDec,
		MarginMode:   "isolated",
	})
	if err != nil {
		writeError(w, 502, fmt.Sprintf("place order: %s", err))
		return
	}

	// Place TP/SL algo orders
	algoOrders, err := s.okx.PlaceTPSL(ctx, &okx.TPSLRequest{
		Symbol:          req.Symbol,
		PositionSide:    posSide,
		Size:            contractsDec,
		TakeProfitPrice: decimal.NewFromFloat(req.TPPrice),
		StopLossPrice:   decimal.NewFromFloat(req.SLPrice),
		MarginMode:      "isolated",
	})
	if err != nil {
		s.log.WithError(err).Warn("PlaceTPSL failed (position is open, manual management required)")
	}

	algoIDs := make([]string, len(algoOrders))
	for i, a := range algoOrders {
		algoIDs[i] = a.AlgoID
	}

	writeJSON(w, 200, map[string]interface{}{
		"order_id":   order.ID,
		"algo_ids":   algoIDs,
		"symbol":     req.Symbol,
		"direction":  req.Direction,
		"contracts":  contracts,
		"size_usdt":  req.SizeUSDT,
		"sl_price":   req.SLPrice,
		"tp_price":   req.TPPrice,
		"entry_price": price,
		"status":     "submitted",
	})
}

type closeOrderReq struct {
	Symbol string `json:"symbol"`
	Reason string `json:"reason"`
}

func (s *Server) handleCloseOrder(w http.ResponseWriter, r *http.Request) {
	var req closeOrderReq
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, 400, "invalid request body")
		return
	}
	if req.Symbol == "" {
		writeError(w, 400, "symbol is required")
		return
	}

	ctx, cancel := s.requestCtx(r)
	defer cancel()

	positions, err := s.okx.GetPositions(ctx)
	if err != nil {
		writeError(w, 502, fmt.Sprintf("get positions: %s", err))
		return
	}

	var pos *okx.Position
	for _, p := range positions {
		if p.Symbol == req.Symbol {
			pos = p
			break
		}
	}

	if pos == nil {
		writeError(w, 404, fmt.Sprintf("no open position for %s", req.Symbol))
		return
	}

	if err := s.okx.ClosePosition(ctx, req.Symbol, pos.Side, "isolated"); err != nil {
		writeError(w, 502, fmt.Sprintf("close position: %s", err))
		return
	}

	writeJSON(w, 200, map[string]interface{}{
		"symbol": req.Symbol,
		"side":   string(pos.Side),
		"reason": req.Reason,
		"status": "closed",
	})
}

func toFloat(d decimal.Decimal) float64 {
	f, _ := d.Float64()
	return f
}

func last(vals []decimal.Decimal) decimal.Decimal {
	if len(vals) == 0 {
		return decimal.Zero
	}
	return vals[len(vals)-1]
}
