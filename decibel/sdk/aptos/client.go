package aptos

import (
	"fmt"
	"math/big"

	aptossdk "github.com/aptos-labs/aptos-go-sdk"
	aptossdkapi "github.com/aptos-labs/aptos-go-sdk/api"
	"github.com/aptos-labs/aptos-go-sdk/bcs"
	aptoscrypto "github.com/aptos-labs/aptos-go-sdk/crypto"
)

const (
	placeOrderModule  = "dex_accounts_entry"
	placeOrderFunc    = "place_order_to_subaccount"
	cancelOrderModule = "dex_accounts"
	cancelOrderFunc   = "cancel_order_to_subaccount"
)

type Client struct {
	account        *aptossdk.Account
	packageAddress aptossdk.AccountAddress
	submitter      submitter
}

func NewClient(privateKey string, opts ...ClientOption) (*Client, error) {
	cfg := clientConfig{
		packageAddress: DefaultPackageAddress,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(&cfg); err != nil {
			return nil, err
		}
	}

	key := &aptoscrypto.Ed25519PrivateKey{}
	if err := key.FromHex(privateKey); err != nil {
		return nil, fmt.Errorf("parse aptos private key: %w", err)
	}

	account, err := aptossdk.NewAccountFromSigner(key)
	if err != nil {
		return nil, fmt.Errorf("derive aptos account: %w", err)
	}

	packageAddress, err := parseAddress(cfg.packageAddress)
	if err != nil {
		return nil, fmt.Errorf("parse package address: %w", err)
	}

	submitter := cfg.submitter
	if submitter == nil {
		submitter, err = aptossdk.NewClient(aptossdk.MainnetConfig)
		if err != nil {
			return nil, fmt.Errorf("create aptos client: %w", err)
		}
	}

	return &Client{
		account:        account,
		packageAddress: packageAddress,
		submitter:      submitter,
	}, nil
}

func (c *Client) AccountAddress() string {
	addr := c.account.AccountAddress()
	return addr.StringLong()
}

func (c *Client) PlaceOrder(req PlaceOrderRequest) (*aptossdkapi.SubmitTransactionResponse, error) {
	payload, err := c.placeOrderPayload(req)
	if err != nil {
		return nil, err
	}
	return c.submitter.BuildSignAndSubmitTransaction(c.account, payload)
}

func (c *Client) CancelOrder(req CancelOrderRequest) (*aptossdkapi.SubmitTransactionResponse, error) {
	payload, err := c.cancelOrderPayload(req)
	if err != nil {
		return nil, err
	}
	return c.submitter.BuildSignAndSubmitTransaction(c.account, payload)
}

func (c *Client) placeOrderPayload(req PlaceOrderRequest) (aptossdk.TransactionPayload, error) {
	if req.Encoder == nil {
		return aptossdk.TransactionPayload{}, fmt.Errorf("order encoder is required")
	}

	subaccountArg, err := addressArg(req.SubaccountAddr)
	if err != nil {
		return aptossdk.TransactionPayload{}, fmt.Errorf("subaccount address: %w", err)
	}
	marketArg, err := addressArg(req.MarketAddr)
	if err != nil {
		return aptossdk.TransactionPayload{}, fmt.Errorf("market address: %w", err)
	}
	builderAddrArg, err := optionAddressArg(req.BuilderAddress)
	if err != nil {
		return aptossdk.TransactionPayload{}, fmt.Errorf("builder address: %w", err)
	}
	priceUnits, err := req.Encoder.EncodePrice(req.Price)
	if err != nil {
		return aptossdk.TransactionPayload{}, fmt.Errorf("encode price: %w", err)
	}
	sizeUnits, err := req.Encoder.EncodeSize(req.Size)
	if err != nil {
		return aptossdk.TransactionPayload{}, fmt.Errorf("encode size: %w", err)
	}

	args := make([][]byte, 0, 15)
	args = append(args,
		subaccountArg,
		marketArg,
		mustSerializeU64(priceUnits),
		mustSerializeU64(sizeUnits),
		mustSerializeBool(req.IsBuy),
		[]byte{byte(req.TimeInForce)},
		mustSerializeBool(req.ReduceOnly),
		mustSerializeOptionString(req.ClientOrderID),
		mustSerializeOptionU64(req.StopPrice),
		mustSerializeOptionU64(req.TakeProfitTriggerPrice),
		mustSerializeOptionU64(req.TakeProfitLimitPrice),
		mustSerializeOptionU64(req.StopLossTriggerPrice),
		mustSerializeOptionU64(req.StopLossLimitPrice),
		builderAddrArg,
		mustSerializeOptionU64(req.BuilderFees),
	)

	return c.entryFunctionPayload(placeOrderModule, placeOrderFunc, args), nil
}

func (c *Client) cancelOrderPayload(req CancelOrderRequest) (aptossdk.TransactionPayload, error) {
	subaccountArg, err := addressArg(req.SubaccountAddr)
	if err != nil {
		return aptossdk.TransactionPayload{}, fmt.Errorf("subaccount address: %w", err)
	}
	orderIDArg, err := u128Arg(req.OrderID)
	if err != nil {
		return aptossdk.TransactionPayload{}, fmt.Errorf("order id: %w", err)
	}
	marketArg, err := addressArg(req.MarketAddr)
	if err != nil {
		return aptossdk.TransactionPayload{}, fmt.Errorf("market address: %w", err)
	}

	return c.entryFunctionPayload(cancelOrderModule, cancelOrderFunc, [][]byte{
		subaccountArg,
		orderIDArg,
		marketArg,
	}), nil
}

func (c *Client) entryFunctionPayload(moduleName, functionName string, args [][]byte) aptossdk.TransactionPayload {
	return aptossdk.TransactionPayload{
		Payload: &aptossdk.EntryFunction{
			Module: aptossdk.ModuleId{
				Address: c.packageAddress,
				Name:    moduleName,
			},
			Function: functionName,
			ArgTypes: []aptossdk.TypeTag{},
			Args:     args,
		},
	}
}

func parseAddress(value string) (aptossdk.AccountAddress, error) {
	addr := aptossdk.AccountAddress{}
	if err := addr.ParseStringRelaxed(value); err != nil {
		return aptossdk.AccountAddress{}, err
	}
	return addr, nil
}

func addressArg(value string) ([]byte, error) {
	addr, err := parseAddress(value)
	if err != nil {
		return nil, err
	}
	return addr[:], nil
}

func u128Arg(value string) ([]byte, error) {
	num, ok := new(big.Int).SetString(value, 10)
	if !ok {
		return nil, fmt.Errorf("invalid decimal %q", value)
	}
	if num.Sign() < 0 {
		return nil, fmt.Errorf("must be non-negative")
	}
	if num.BitLen() > 128 {
		return nil, fmt.Errorf("exceeds u128")
	}
	return bcs.SerializeU128(*num)
}

func optionAddressArg(value *string) ([]byte, error) {
	if value == nil {
		return serializeOptionAddress(nil), nil
	}
	addr, err := parseAddress(*value)
	if err != nil {
		return nil, err
	}
	return serializeOptionAddress(&addr), nil
}

func mustSerializeU64(value uint64) []byte {
	out, err := bcs.SerializeU64(value)
	if err != nil {
		panic(err)
	}
	return out
}

func mustSerializeBool(value bool) []byte {
	out, err := bcs.SerializeBool(value)
	if err != nil {
		panic(err)
	}
	return out
}

func mustSerializeOptionString(value *string) []byte {
	return serializeOption(value, func(ser *bcs.Serializer, item string) {
		ser.WriteString(item)
	})
}

func mustSerializeOptionU64(value *uint64) []byte {
	return serializeOption(value, func(ser *bcs.Serializer, item uint64) {
		ser.U64(item)
	})
}

func serializeOptionAddress(value *aptossdk.AccountAddress) []byte {
	return serializeOption(value, func(ser *bcs.Serializer, item aptossdk.AccountAddress) {
		ser.FixedBytes(item[:])
	})
}

func serializeOption[T any](value *T, fn func(ser *bcs.Serializer, item T)) []byte {
	ser := &bcs.Serializer{}
	bcs.SerializeOption(ser, value, fn)
	if err := ser.Error(); err != nil {
		panic(err)
	}
	return ser.ToBytes()
}
