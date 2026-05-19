package indicators

import (
	"github.com/beto/trading-agent/market-api/internal/okx"
	"github.com/shopspring/decimal"
)

func EMA(candles []okx.Candle, period int) []decimal.Decimal {
	if len(candles) < period {
		return nil
	}

	result := make([]decimal.Decimal, len(candles))
	mult := decimal.NewFromFloat(2.0 / float64(period+1))

	sum := decimal.Zero
	for i := 0; i < period; i++ {
		sum = sum.Add(candles[i].Close)
	}
	result[period-1] = sum.Div(decimal.NewFromInt(int64(period)))

	for i := period; i < len(candles); i++ {
		result[i] = candles[i].Close.Sub(result[i-1]).Mul(mult).Add(result[i-1])
	}
	return result
}

func RSI(candles []okx.Candle, period int) []decimal.Decimal {
	if len(candles) < period+1 {
		return nil
	}

	gains := make([]decimal.Decimal, len(candles))
	losses := make([]decimal.Decimal, len(candles))
	for i := 1; i < len(candles); i++ {
		diff := candles[i].Close.Sub(candles[i-1].Close)
		if diff.IsPositive() {
			gains[i] = diff
		} else {
			losses[i] = diff.Abs()
		}
	}

	avgGain, avgLoss := decimal.Zero, decimal.Zero
	for i := 1; i <= period; i++ {
		avgGain = avgGain.Add(gains[i])
		avgLoss = avgLoss.Add(losses[i])
	}
	p := decimal.NewFromInt(int64(period))
	avgGain = avgGain.Div(p)
	avgLoss = avgLoss.Div(p)

	result := make([]decimal.Decimal, len(candles))
	calcRSI := func(g, l decimal.Decimal) decimal.Decimal {
		if l.IsZero() {
			return decimal.NewFromInt(100)
		}
		return decimal.NewFromInt(100).Sub(
			decimal.NewFromInt(100).Div(decimal.NewFromInt(1).Add(g.Div(l))),
		)
	}
	result[period] = calcRSI(avgGain, avgLoss)

	for i := period + 1; i < len(candles); i++ {
		avgGain = avgGain.Mul(p.Sub(decimal.NewFromInt(1))).Add(gains[i]).Div(p)
		avgLoss = avgLoss.Mul(p.Sub(decimal.NewFromInt(1))).Add(losses[i]).Div(p)
		result[i] = calcRSI(avgGain, avgLoss)
	}
	return result
}

func ATR(candles []okx.Candle, period int) []decimal.Decimal {
	if len(candles) < period+1 {
		return nil
	}

	tr := make([]decimal.Decimal, len(candles))
	for i := 1; i < len(candles); i++ {
		hl := candles[i].High.Sub(candles[i].Low)
		hc := candles[i].High.Sub(candles[i-1].Close).Abs()
		lc := candles[i].Low.Sub(candles[i-1].Close).Abs()
		tr[i] = hl
		if hc.GreaterThan(tr[i]) {
			tr[i] = hc
		}
		if lc.GreaterThan(tr[i]) {
			tr[i] = lc
		}
	}

	p := decimal.NewFromInt(int64(period))
	result := make([]decimal.Decimal, len(candles))
	sum := decimal.Zero
	for i := 1; i <= period; i++ {
		sum = sum.Add(tr[i])
	}
	result[period] = sum.Div(p)

	for i := period + 1; i < len(candles); i++ {
		result[i] = result[i-1].Mul(p.Sub(decimal.NewFromInt(1))).Add(tr[i]).Div(p)
	}
	return result
}

func ADX(candles []okx.Candle, period int) []decimal.Decimal {
	if len(candles) < period*2 {
		return nil
	}

	plusDM := make([]decimal.Decimal, len(candles))
	minusDM := make([]decimal.Decimal, len(candles))
	tr := make([]decimal.Decimal, len(candles))

	for i := 1; i < len(candles); i++ {
		up := candles[i].High.Sub(candles[i-1].High)
		down := candles[i-1].Low.Sub(candles[i].Low)
		if up.GreaterThan(down) && up.IsPositive() {
			plusDM[i] = up
		}
		if down.GreaterThan(up) && down.IsPositive() {
			minusDM[i] = down
		}
		hl := candles[i].High.Sub(candles[i].Low)
		hc := candles[i].High.Sub(candles[i-1].Close).Abs()
		lc := candles[i].Low.Sub(candles[i-1].Close).Abs()
		tr[i] = hl
		if hc.GreaterThan(tr[i]) {
			tr[i] = hc
		}
		if lc.GreaterThan(tr[i]) {
			tr[i] = lc
		}
	}

	p := decimal.NewFromInt(int64(period))
	w := p.Sub(decimal.NewFromInt(1))

	sPlusDM := make([]decimal.Decimal, len(candles))
	sMinusDM := make([]decimal.Decimal, len(candles))
	sTR := make([]decimal.Decimal, len(candles))

	sum1, sum2, sum3 := decimal.Zero, decimal.Zero, decimal.Zero
	for i := 1; i <= period; i++ {
		sum1 = sum1.Add(plusDM[i])
		sum2 = sum2.Add(minusDM[i])
		sum3 = sum3.Add(tr[i])
	}
	sPlusDM[period] = sum1
	sMinusDM[period] = sum2
	sTR[period] = sum3

	for i := period + 1; i < len(candles); i++ {
		sPlusDM[i] = sPlusDM[i-1].Mul(w).Add(plusDM[i]).Div(p)
		sMinusDM[i] = sMinusDM[i-1].Mul(w).Add(minusDM[i]).Div(p)
		sTR[i] = sTR[i-1].Mul(w).Add(tr[i]).Div(p)
	}

	dx := make([]decimal.Decimal, len(candles))
	for i := period; i < len(candles); i++ {
		if sTR[i].IsZero() {
			continue
		}
		pdi := decimal.NewFromInt(100).Mul(sPlusDM[i]).Div(sTR[i])
		mdi := decimal.NewFromInt(100).Mul(sMinusDM[i]).Div(sTR[i])
		sum := pdi.Add(mdi)
		diff := pdi.Sub(mdi).Abs()
		if !sum.IsZero() {
			dx[i] = decimal.NewFromInt(100).Mul(diff).Div(sum)
		}
	}

	result := make([]decimal.Decimal, len(candles))
	dxSum := decimal.Zero
	for i := period; i < period*2; i++ {
		dxSum = dxSum.Add(dx[i])
	}
	result[period*2-1] = dxSum.Div(p)

	for i := period * 2; i < len(candles); i++ {
		result[i] = result[i-1].Mul(w).Add(dx[i]).Div(p)
	}
	return result
}

// CrossDirection returns "bullish", "bearish", or "none" for the most recent
// EMA9/21 cross in the last maxCandlesAgo candles, plus how many candles ago.
func CrossDirection(ema9, ema21 []decimal.Decimal, maxCandlesAgo int) (direction string, candlesAgo int) {
	n := len(ema9)
	if n < 2 || len(ema21) < 2 {
		return "none", -1
	}

	limit := maxCandlesAgo
	if limit >= n {
		limit = n - 1
	}

	for ago := 0; ago < limit; ago++ {
		cur := n - 1 - ago
		prev := cur - 1
		if prev < 0 {
			break
		}
		if ema9[prev].IsZero() || ema21[prev].IsZero() || ema9[cur].IsZero() || ema21[cur].IsZero() {
			continue
		}
		wasBelowOrEqual := ema9[prev].LessThanOrEqual(ema21[prev])
		isAbove := ema9[cur].GreaterThan(ema21[cur])
		if wasBelowOrEqual && isAbove {
			return "bullish", ago
		}
		wasAboveOrEqual := ema9[prev].GreaterThanOrEqual(ema21[prev])
		isBelow := ema9[cur].LessThan(ema21[cur])
		if wasAboveOrEqual && isBelow {
			return "bearish", ago
		}
	}
	return "none", -1
}
