package aptos

import (
	aptossdk "github.com/aptos-labs/aptos-go-sdk"
	aptossdkapi "github.com/aptos-labs/aptos-go-sdk/api"
	"github.com/shopspring/decimal"
)

const DefaultPackageAddress = "0x50ead22afd6ffd9769e3b3d6e0e64a2a350d68e8b102c4e72e33d0b8cfdfdb06"

type TimeInForce uint8

const (
	TimeInForceGoodTillCancelled TimeInForce = iota
	TimeInForcePostOnly
	TimeInForceImmediateOrCancel
)

type PlaceOrderRequest struct {
	SubaccountAddr         string
	MarketAddr             string
	Price                  decimal.Decimal
	Size                   decimal.Decimal
	Encoder                OrderEncoder
	IsBuy                  bool
	TimeInForce            TimeInForce
	ReduceOnly             bool
	ClientOrderID          *string
	StopPrice              *uint64
	TakeProfitTriggerPrice *uint64
	TakeProfitLimitPrice   *uint64
	StopLossTriggerPrice   *uint64
	StopLossLimitPrice     *uint64
	BuilderAddress         *string
	BuilderFees            *uint64
}

type CancelOrderRequest struct {
	SubaccountAddr string
	OrderID        string
	MarketAddr     string
}

type OrderEncoder interface {
	EncodePrice(price decimal.Decimal) (uint64, error)
	EncodeSize(size decimal.Decimal) (uint64, error)
}

type ClientOption func(*clientConfig) error

type submitter interface {
	BuildSignAndSubmitTransaction(
		sender aptossdk.TransactionSigner,
		payload aptossdk.TransactionPayload,
		options ...any,
	) (*aptossdkapi.SubmitTransactionResponse, error)
}

type clientConfig struct {
	packageAddress string
	submitter      submitter
}

func WithPackageAddress(address string) ClientOption {
	return func(cfg *clientConfig) error {
		cfg.packageAddress = address
		return nil
	}
}

func WithSubmitter(s submitter) ClientOption {
	return func(cfg *clientConfig) error {
		cfg.submitter = s
		return nil
	}
}
