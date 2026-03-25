package aptos

import (
	"errors"
	"math/big"
	"testing"

	aptossdk "github.com/aptos-labs/aptos-go-sdk"
	"github.com/aptos-labs/aptos-go-sdk/api"
	"github.com/aptos-labs/aptos-go-sdk/bcs"
	aptoscrypto "github.com/aptos-labs/aptos-go-sdk/crypto"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

const (
	testPrivateKey = "ed25519-priv-0xc5338cd251c22daa8c9c9cc94f498cc8a5c7e1d2e75287a5dda91096fe64efa5"
	testPackage    = "0x50ead22afd6ffd9769e3b3d6e0e64a2a350d68e8b102c4e72e33d0b8cfdfdb06"
)

type fakeSubmitter struct {
	senderAddr string
	payload    aptossdk.TransactionPayload
	calls      int
}

func (f *fakeSubmitter) BuildSignAndSubmitTransaction(
	sender aptossdk.TransactionSigner,
	payload aptossdk.TransactionPayload,
	options ...any,
) (*api.SubmitTransactionResponse, error) {
	f.calls++
	senderAddr := sender.AccountAddress()
	f.senderAddr = senderAddr.StringLong()
	f.payload = payload
	return &api.SubmitTransactionResponse{Hash: "0xfeedbeef"}, nil
}

type fakeOrderEncoder struct {
	priceInput   decimal.Decimal
	sizeInput    decimal.Decimal
	encodedPrice uint64
	encodedSize  uint64
	err          error
}

func (f *fakeOrderEncoder) EncodePrice(price decimal.Decimal) (uint64, error) {
	f.priceInput = price
	if f.err != nil {
		return 0, f.err
	}
	return f.encodedPrice, nil
}

func (f *fakeOrderEncoder) EncodeSize(size decimal.Decimal) (uint64, error) {
	f.sizeInput = size
	if f.err != nil {
		return 0, f.err
	}
	return f.encodedSize, nil
}

func TestDecibelAptosPlaceOrderEncodesPayloadAndSubmits(t *testing.T) {
	submitter := &fakeSubmitter{}

	client, err := NewClient(
		testPrivateKey,
		WithPackageAddress(testPackage),
		WithSubmitter(submitter),
	)
	require.NoError(t, err)

	expectedSigner := mustAccountAddressForKey(t, testPrivateKey)
	require.Equal(t, expectedSigner, client.AccountAddress())

	clientOrderID := "cid-123"
	builderAddr := "0x789"
	builderFees := uint64(42)
	encoder := &fakeOrderEncoder{
		encodedPrice: 5_670_000_000,
		encodedSize:  1_000_000_000,
	}

	resp, err := client.PlaceOrder(PlaceOrderRequest{
		SubaccountAddr: "0x123",
		MarketAddr:     "0x456",
		Price:          decimal.RequireFromString("5670"),
		Size:           decimal.RequireFromString("1"),
		Encoder:        encoder,
		IsBuy:          true,
		TimeInForce:    TimeInForceGoodTillCancelled,
		ReduceOnly:     false,
		ClientOrderID:  &clientOrderID,
		BuilderAddress: &builderAddr,
		BuilderFees:    &builderFees,
	})
	require.NoError(t, err)
	require.Equal(t, "0xfeedbeef", resp.Hash)
	require.Equal(t, expectedSigner, submitter.senderAddr)
	require.True(t, decimal.RequireFromString("5670").Equal(encoder.priceInput))
	require.True(t, decimal.RequireFromString("1").Equal(encoder.sizeInput))

	entry := requireEntryFunction(t, submitter.payload)
	require.Equal(t, testPackage, entry.Module.Address.StringLong())
	require.Equal(t, "dex_accounts_entry", entry.Module.Name)
	require.Equal(t, "place_order_to_subaccount", entry.Function)
	require.Len(t, entry.Args, 15)

	require.Equal(t, mustAddressBytes(t, "0x123"), entry.Args[0])
	require.Equal(t, mustAddressBytes(t, "0x456"), entry.Args[1])
	require.Equal(t, mustU64Bytes(t, 5_670_000_000), entry.Args[2])
	require.Equal(t, mustU64Bytes(t, 1_000_000_000), entry.Args[3])
	require.Equal(t, mustBoolBytes(t, true), entry.Args[4])
	require.Equal(t, []byte{0x00}, entry.Args[5])
	require.Equal(t, mustBoolBytes(t, false), entry.Args[6])
	require.Equal(t, mustOptionStringBytes(t, &clientOrderID), entry.Args[7])
	require.Equal(t, mustOptionU64Bytes(t, nil), entry.Args[8])
	require.Equal(t, mustOptionU64Bytes(t, nil), entry.Args[9])
	require.Equal(t, mustOptionU64Bytes(t, nil), entry.Args[10])
	require.Equal(t, mustOptionU64Bytes(t, nil), entry.Args[11])
	require.Equal(t, mustOptionU64Bytes(t, nil), entry.Args[12])
	require.Equal(t, mustOptionAddressBytes(t, &builderAddr), entry.Args[13])
	require.Equal(t, mustOptionU64Bytes(t, &builderFees), entry.Args[14])
}

func TestDecibelAptosPlaceOrderPropagatesEncoderErrorBeforeSubmit(t *testing.T) {
	submitter := &fakeSubmitter{}
	encoderErr := errors.New("invalid precision")
	encoder := &fakeOrderEncoder{err: encoderErr}

	client, err := NewClient(
		testPrivateKey,
		WithPackageAddress(testPackage),
		WithSubmitter(submitter),
	)
	require.NoError(t, err)

	_, err = client.PlaceOrder(PlaceOrderRequest{
		SubaccountAddr: "0x123",
		MarketAddr:     "0x456",
		Price:          decimal.RequireFromString("12.387"),
		Size:           decimal.RequireFromString("1"),
		Encoder:        encoder,
		IsBuy:          true,
		TimeInForce:    TimeInForceGoodTillCancelled,
	})
	require.ErrorIs(t, err, encoderErr)
	require.Equal(t, 0, submitter.calls)
	require.True(t, decimal.RequireFromString("12.387").Equal(encoder.priceInput))
	require.True(t, encoder.sizeInput.IsZero())
}

func TestDecibelAptosCancelOrderEncodesPayloadAndSubmits(t *testing.T) {
	submitter := &fakeSubmitter{}

	client, err := NewClient(
		testPrivateKey,
		WithPackageAddress(testPackage),
		WithSubmitter(submitter),
	)
	require.NoError(t, err)

	resp, err := client.CancelOrder(CancelOrderRequest{
		SubaccountAddr: "0x123",
		MarketAddr:     "0x456",
		OrderID:        "12345678901234567890",
	})
	require.NoError(t, err)
	require.Equal(t, "0xfeedbeef", resp.Hash)

	entry := requireEntryFunction(t, submitter.payload)
	require.Equal(t, testPackage, entry.Module.Address.StringLong())
	require.Equal(t, "dex_accounts", entry.Module.Name)
	require.Equal(t, "cancel_order_to_subaccount", entry.Function)
	require.Len(t, entry.Args, 3)
	require.Equal(t, mustAddressBytes(t, "0x123"), entry.Args[0])
	require.Equal(t, mustU128Bytes(t, "12345678901234567890"), entry.Args[1])
	require.Equal(t, mustAddressBytes(t, "0x456"), entry.Args[2])
}

func requireEntryFunction(t *testing.T, payload aptossdk.TransactionPayload) *aptossdk.EntryFunction {
	t.Helper()

	entry, ok := payload.Payload.(*aptossdk.EntryFunction)
	require.True(t, ok)
	return entry
}

func mustAccountAddressForKey(t *testing.T, privateKey string) string {
	t.Helper()

	key := &aptoscrypto.Ed25519PrivateKey{}
	require.NoError(t, key.FromHex(privateKey))

	account, err := aptossdk.NewAccountFromSigner(key)
	require.NoError(t, err)
	addr := account.AccountAddress()
	return addr.StringLong()
}

func mustAddressBytes(t *testing.T, addr string) []byte {
	t.Helper()

	accountAddr := mustParseAddress(t, addr)
	return accountAddr[:]
}

func mustParseAddress(t *testing.T, value string) aptossdk.AccountAddress {
	t.Helper()

	addr := aptossdk.AccountAddress{}
	require.NoError(t, addr.ParseStringRelaxed(value))
	return addr
}

func mustU64Bytes(t *testing.T, value uint64) []byte {
	t.Helper()

	bytes, err := bcs.SerializeU64(value)
	require.NoError(t, err)
	return bytes
}

func mustU128Bytes(t *testing.T, value string) []byte {
	t.Helper()

	num, ok := new(big.Int).SetString(value, 10)
	require.True(t, ok)

	bytes, err := bcs.SerializeU128(*num)
	require.NoError(t, err)
	return bytes
}

func mustBoolBytes(t *testing.T, value bool) []byte {
	t.Helper()

	bytes, err := bcs.SerializeBool(value)
	require.NoError(t, err)
	return bytes
}

func mustOptionStringBytes(t *testing.T, value *string) []byte {
	t.Helper()

	ser := &bcs.Serializer{}
	bcs.SerializeOption(ser, value, func(ser *bcs.Serializer, item string) {
		ser.WriteString(item)
	})
	require.NoError(t, ser.Error())
	return ser.ToBytes()
}

func mustOptionU64Bytes(t *testing.T, value *uint64) []byte {
	t.Helper()

	ser := &bcs.Serializer{}
	bcs.SerializeOption(ser, value, func(ser *bcs.Serializer, item uint64) {
		ser.U64(item)
	})
	require.NoError(t, ser.Error())
	return ser.ToBytes()
}

func mustOptionAddressBytes(t *testing.T, value *string) []byte {
	t.Helper()

	ser := &bcs.Serializer{}
	if value == nil {
		bcs.SerializeOption[aptossdk.AccountAddress](ser, nil, func(ser *bcs.Serializer, item aptossdk.AccountAddress) {
			ser.FixedBytes(item[:])
		})
		require.NoError(t, ser.Error())
		return ser.ToBytes()
	}

	addr := mustParseAddress(t, *value)
	bcs.SerializeOption(ser, &addr, func(ser *bcs.Serializer, item aptossdk.AccountAddress) {
		ser.FixedBytes(item[:])
	})
	require.NoError(t, ser.Error())
	return ser.ToBytes()
}
