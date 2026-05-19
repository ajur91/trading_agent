package okx

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"
	"github.com/sirupsen/logrus"
)

const (
	baseURL              = "https://www.okx.com"
	maxRequestsPerSecond = 15
)

type Config struct {
	APIKey     string
	SecretKey  string
	Passphrase string
	IsDemo     bool
}

type Client struct {
	cfg         *Config
	http        *http.Client
	log         *logrus.Logger
	rateLimiter *rateLimiter
}

type rateLimiter struct {
	tokens   chan struct{}
	ticker   *time.Ticker
	stopChan chan struct{}
}

func newRateLimiter(rps int) *rateLimiter {
	rl := &rateLimiter{
		tokens:   make(chan struct{}, rps),
		ticker:   time.NewTicker(time.Second / time.Duration(rps)),
		stopChan: make(chan struct{}),
	}
	for i := 0; i < rps; i++ {
		rl.tokens <- struct{}{}
	}
	go func() {
		for {
			select {
			case <-rl.stopChan:
				rl.ticker.Stop()
				return
			case <-rl.ticker.C:
				select {
				case rl.tokens <- struct{}{}:
				default:
				}
			}
		}
	}()
	return rl
}

func (rl *rateLimiter) acquire(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-rl.tokens:
		return nil
	}
}

func (rl *rateLimiter) stop() {
	close(rl.stopChan)
}

func NewClient(cfg *Config, log *logrus.Logger) *Client {
	return &Client{
		cfg:         cfg,
		http:        &http.Client{Timeout: 30 * time.Second},
		log:         log,
		rateLimiter: newRateLimiter(maxRequestsPerSecond),
	}
}

func (c *Client) Close() {
	c.rateLimiter.stop()
}

func (c *Client) Ping(ctx context.Context) error {
	_, err := c.GetBalance(ctx)
	return err
}

func (c *Client) sign(timestamp, method, path, body string) string {
	msg := timestamp + method + path + body
	h := hmac.New(sha256.New, []byte(c.cfg.SecretKey))
	h.Write([]byte(msg))
	return base64.StdEncoding.EncodeToString(h.Sum(nil))
}

func (c *Client) do(ctx context.Context, method, path string, body interface{}) (json.RawMessage, error) {
	if err := c.rateLimiter.acquire(ctx); err != nil {
		return nil, err
	}

	var bodyStr string
	var bodyReader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		bodyStr = string(b)
		bodyReader = strings.NewReader(bodyStr)
	}

	req, err := http.NewRequestWithContext(ctx, method, baseURL+path, bodyReader)
	if err != nil {
		return nil, err
	}

	ts := time.Now().UTC().Format("2006-01-02T15:04:05.000Z")
	req.Header.Set("OK-ACCESS-KEY", c.cfg.APIKey)
	req.Header.Set("OK-ACCESS-SIGN", c.sign(ts, method, path, bodyStr))
	req.Header.Set("OK-ACCESS-TIMESTAMP", ts)
	req.Header.Set("OK-ACCESS-PASSPHRASE", c.cfg.Passphrase)
	req.Header.Set("Content-Type", "application/json")
	if c.cfg.IsDemo {
		req.Header.Set("x-simulated-trading", "1")
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)

	var envelope struct {
		Code string          `json:"code"`
		Msg  string          `json:"msg"`
		Data json.RawMessage `json:"data"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return nil, fmt.Errorf("invalid response: %w", err)
	}
	if envelope.Code != "0" {
		var details []struct {
			SMsg string `json:"sMsg"`
		}
		json.Unmarshal(envelope.Data, &details)
		detail := ""
		if len(details) > 0 && details[0].SMsg != "" {
			detail = " — " + details[0].SMsg
		}
		return nil, fmt.Errorf("OKX [%s]: %s%s", envelope.Code, envelope.Msg, detail)
	}

	return envelope.Data, nil
}

func (c *Client) GetTicker(ctx context.Context, symbol string) (*Ticker, error) {
	data, err := c.do(ctx, "GET", fmt.Sprintf("/api/v5/market/ticker?instId=%s", symbol), nil)
	if err != nil {
		return nil, err
	}

	var tickers []struct {
		InstID string `json:"instId"`
		Last   string `json:"last"`
		BidPx  string `json:"bidPx"`
		AskPx  string `json:"askPx"`
		Vol24h string `json:"vol24h"`
		Ts     string `json:"ts"`
	}
	json.Unmarshal(data, &tickers)

	if len(tickers) == 0 {
		return nil, fmt.Errorf("ticker not found: %s", symbol)
	}

	t := tickers[0]
	ts, _ := strconv.ParseInt(t.Ts, 10, 64)
	return &Ticker{
		Symbol:    t.InstID,
		Last:      parseDecimal(t.Last),
		Bid:       parseDecimal(t.BidPx),
		Ask:       parseDecimal(t.AskPx),
		Volume24h: parseDecimal(t.Vol24h),
		Timestamp: time.UnixMilli(ts),
	}, nil
}

func (c *Client) GetCandles(ctx context.Context, symbol, bar string, limit int) ([]Candle, error) {
	data, err := c.do(ctx, "GET",
		fmt.Sprintf("/api/v5/market/candles?instId=%s&bar=%s&limit=%d", symbol, bar, limit), nil)
	if err != nil {
		return nil, err
	}

	var raw [][]string
	json.Unmarshal(data, &raw)

	candles := make([]Candle, len(raw))
	for i, r := range raw {
		if len(r) < 6 {
			continue
		}
		ts, _ := strconv.ParseInt(r[0], 10, 64)
		candles[i] = Candle{
			Timestamp: time.UnixMilli(ts),
			Open:      parseDecimal(r[1]),
			High:      parseDecimal(r[2]),
			Low:       parseDecimal(r[3]),
			Close:     parseDecimal(r[4]),
			Volume:    parseDecimal(r[5]),
		}
	}

	// OKX returns newest first; reverse to chronological order
	for i, j := 0, len(candles)-1; i < j; i, j = i+1, j-1 {
		candles[i], candles[j] = candles[j], candles[i]
	}
	return candles, nil
}

func (c *Client) GetBalance(ctx context.Context) (*Balance, error) {
	data, err := c.do(ctx, "GET", "/api/v5/account/balance", nil)
	if err != nil {
		return nil, err
	}

	var balances []struct {
		Details []struct {
			Ccy      string `json:"ccy"`
			CashBal  string `json:"cashBal"`
			AvailBal string `json:"availBal"`
			FrzBal   string `json:"frozenBal"`
		} `json:"details"`
	}
	json.Unmarshal(data, &balances)

	if len(balances) == 0 {
		return &Balance{Currency: "USDT"}, nil
	}

	for _, d := range balances[0].Details {
		if d.Ccy == "USDT" {
			return &Balance{
				Currency:  "USDT",
				Total:     parseDecimal(d.CashBal),
				Available: parseDecimal(d.AvailBal),
				Frozen:    parseDecimal(d.FrzBal),
			}, nil
		}
	}
	return &Balance{Currency: "USDT"}, nil
}

func (c *Client) GetPositions(ctx context.Context) ([]*Position, error) {
	data, err := c.do(ctx, "GET", "/api/v5/account/positions?instType=SWAP", nil)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		InstID  string `json:"instId"`
		PosSide string `json:"posSide"`
		Pos     string `json:"pos"`
		AvgPx   string `json:"avgPx"`
		MarkPx  string `json:"markPx"`
		Lever   string `json:"lever"`
		Upl     string `json:"upl"`
	}
	json.Unmarshal(data, &raw)

	var positions []*Position
	for _, p := range raw {
		size := parseDecimal(p.Pos)
		if size.IsZero() {
			continue
		}
		lever, _ := strconv.Atoi(p.Lever)
		positions = append(positions, &Position{
			Symbol:        p.InstID,
			Side:          PositionSide(p.PosSide),
			Size:          size.Abs(),
			EntryPrice:    parseDecimal(p.AvgPx),
			MarkPrice:     parseDecimal(p.MarkPx),
			Leverage:      lever,
			UnrealizedPnL: parseDecimal(p.Upl),
		})
	}
	return positions, nil
}

func (c *Client) GetInstrument(ctx context.Context, symbol string) (*Instrument, error) {
	data, err := c.do(ctx, "GET",
		fmt.Sprintf("/api/v5/public/instruments?instType=SWAP&instId=%s", symbol), nil)
	if err != nil {
		return nil, err
	}

	var raw []struct {
		InstID string `json:"instId"`
		CtVal  string `json:"ctVal"`
		MinSz  string `json:"minSz"`
		LotSz  string `json:"lotSz"`
		TickSz string `json:"tickSz"`
	}
	json.Unmarshal(data, &raw)

	if len(raw) == 0 {
		return nil, fmt.Errorf("instrument not found: %s", symbol)
	}

	r := raw[0]
	return &Instrument{
		Symbol:        r.InstID,
		ContractValue: parseDecimal(r.CtVal),
		MinSize:       parseDecimal(r.MinSz),
		LotSize:       parseDecimal(r.LotSz),
		TickSize:      parseDecimal(r.TickSz),
	}, nil
}

func (c *Client) SetLeverage(ctx context.Context, symbol string, leverage int, marginMode string) error {
	if marginMode == "" {
		marginMode = "isolated"
	}
	if marginMode == "isolated" {
		c.do(ctx, "POST", "/api/v5/account/set-leverage", map[string]interface{}{
			"instId": symbol, "lever": strconv.Itoa(leverage), "mgnMode": marginMode, "posSide": "long",
		})
		c.do(ctx, "POST", "/api/v5/account/set-leverage", map[string]interface{}{
			"instId": symbol, "lever": strconv.Itoa(leverage), "mgnMode": marginMode, "posSide": "short",
		})
		return nil
	}
	_, err := c.do(ctx, "POST", "/api/v5/account/set-leverage", map[string]interface{}{
		"instId": symbol, "lever": strconv.Itoa(leverage), "mgnMode": marginMode,
	})
	return err
}

func (c *Client) PlaceMarketOrder(ctx context.Context, req *OrderRequest) (*Order, error) {
	if req.MarginMode == "" {
		req.MarginMode = "isolated"
	}
	body := map[string]interface{}{
		"instId":  req.Symbol,
		"tdMode":  req.MarginMode,
		"side":    string(req.Side),
		"posSide": string(req.PositionSide),
		"ordType": "market",
		"sz":      req.Size.String(),
	}

	data, err := c.do(ctx, "POST", "/api/v5/trade/order", body)
	if err != nil {
		return nil, err
	}

	var orders []struct {
		OrdID string `json:"ordId"`
		SCode string `json:"sCode"`
		SMsg  string `json:"sMsg"`
	}
	json.Unmarshal(data, &orders)

	if len(orders) == 0 || orders[0].SCode != "0" {
		msg := "unknown error"
		if len(orders) > 0 {
			msg = orders[0].SMsg
		}
		return nil, fmt.Errorf("place order failed: %s", msg)
	}

	return &Order{
		ID:           orders[0].OrdID,
		Symbol:       req.Symbol,
		Side:         req.Side,
		PositionSide: req.PositionSide,
		Type:         OrderTypeMarket,
		Size:         req.Size,
		Status:       "submitted",
		CreatedAt:    time.Now(),
	}, nil
}

func (c *Client) PlaceTPSL(ctx context.Context, req *TPSLRequest) ([]*AlgoOrder, error) {
	var result []*AlgoOrder
	closeSide := SideSell
	if req.PositionSide == PositionShort {
		closeSide = SideBuy
	}
	if req.MarginMode == "" {
		req.MarginMode = "isolated"
	}

	if !req.TakeProfitPrice.IsZero() {
		o, err := c.placeAlgoOrder(ctx, req.Symbol, closeSide, req.PositionSide, req.Size, req.TakeProfitPrice, "TP", req.MarginMode)
		if err != nil {
			return result, fmt.Errorf("TP order: %w", err)
		}
		result = append(result, o)
	}

	if !req.StopLossPrice.IsZero() {
		o, err := c.placeAlgoOrder(ctx, req.Symbol, closeSide, req.PositionSide, req.Size, req.StopLossPrice, "SL", req.MarginMode)
		if err != nil {
			return result, fmt.Errorf("SL order: %w", err)
		}
		result = append(result, o)
	}

	return result, nil
}

func (c *Client) placeAlgoOrder(ctx context.Context, symbol string, side Side, posSide PositionSide, size, triggerPrice decimal.Decimal, tag, marginMode string) (*AlgoOrder, error) {
	body := map[string]interface{}{
		"instId":  symbol,
		"tdMode":  marginMode,
		"side":    string(side),
		"posSide": string(posSide),
		"ordType": "conditional",
		"sz":      size.String(),
	}

	if tag == "SL" {
		body["slTriggerPx"] = triggerPrice.String()
		body["slOrdPx"] = "-1"
		body["slTriggerPxType"] = "last"
	} else {
		body["tpTriggerPx"] = triggerPrice.String()
		body["tpOrdPx"] = "-1"
		body["tpTriggerPxType"] = "last"
	}

	data, err := c.do(ctx, "POST", "/api/v5/trade/order-algo", body)
	if err != nil {
		return nil, err
	}

	var orders []struct {
		AlgoID string `json:"algoId"`
		SCode  string `json:"sCode"`
		SMsg   string `json:"sMsg"`
	}
	json.Unmarshal(data, &orders)

	if len(orders) == 0 || orders[0].SCode != "0" {
		msg := "unknown error"
		if len(orders) > 0 {
			msg = orders[0].SMsg
		}
		return nil, fmt.Errorf("%s: %s", tag, msg)
	}

	return &AlgoOrder{
		AlgoID:       orders[0].AlgoID,
		Symbol:       symbol,
		OrderType:    tag,
		Side:         side,
		PositionSide: posSide,
		Size:         size,
		TriggerPrice: triggerPrice,
		Status:       "live",
	}, nil
}

func (c *Client) ClosePosition(ctx context.Context, symbol string, side PositionSide, marginMode string) error {
	if marginMode == "" {
		marginMode = "isolated"
	}
	_, err := c.do(ctx, "POST", "/api/v5/trade/close-position", map[string]interface{}{
		"instId":  symbol,
		"mgnMode": marginMode,
		"posSide": string(side),
	})
	return err
}

func parseDecimal(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}
