package okx

import (
	"time"

	"github.com/shopspring/decimal"
)

type Side string
type OrderType string
type PositionSide string

const (
	SideBuy  Side = "buy"
	SideSell Side = "sell"

	OrderTypeMarket OrderType = "market"

	PositionLong  PositionSide = "long"
	PositionShort PositionSide = "short"
)

type Candle struct {
	Timestamp time.Time
	Open      decimal.Decimal
	High      decimal.Decimal
	Low       decimal.Decimal
	Close     decimal.Decimal
	Volume    decimal.Decimal
}

type Ticker struct {
	Symbol    string
	Last      decimal.Decimal
	Bid       decimal.Decimal
	Ask       decimal.Decimal
	Volume24h decimal.Decimal
	Timestamp time.Time
}

type Position struct {
	Symbol        string
	Side          PositionSide
	Size          decimal.Decimal
	EntryPrice    decimal.Decimal
	MarkPrice     decimal.Decimal
	Leverage      int
	UnrealizedPnL decimal.Decimal
}

type Balance struct {
	Currency  string
	Total     decimal.Decimal
	Available decimal.Decimal
	Frozen    decimal.Decimal
}

type Order struct {
	ID           string
	Symbol       string
	Side         Side
	PositionSide PositionSide
	Type         OrderType
	Size         decimal.Decimal
	Status       string
	CreatedAt    time.Time
}

type AlgoOrder struct {
	AlgoID       string
	Symbol       string
	OrderType    string // "TP" or "SL"
	Side         Side
	PositionSide PositionSide
	Size         decimal.Decimal
	TriggerPrice decimal.Decimal
	Status       string
}

type Instrument struct {
	Symbol        string
	ContractValue decimal.Decimal
	MinSize       decimal.Decimal
	LotSize       decimal.Decimal
	TickSize      decimal.Decimal
}

type OrderRequest struct {
	Symbol       string
	Side         Side
	PositionSide PositionSide
	Size         decimal.Decimal
	MarginMode   string
}

type TPSLRequest struct {
	Symbol          string
	PositionSide    PositionSide
	Size            decimal.Decimal
	TakeProfitPrice decimal.Decimal
	StopLossPrice   decimal.Decimal
	MarginMode      string
}
