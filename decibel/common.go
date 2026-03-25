package decibel

import (
	"errors"
	"fmt"
	"math"
	"regexp"
	"sort"
	"strconv"
	"strings"

	exchanges "github.com/QuantProcessing/exchanges"
	decibelrest "github.com/QuantProcessing/exchanges/decibel/sdk/rest"
	decibelws "github.com/QuantProcessing/exchanges/decibel/sdk/ws"
	"github.com/shopspring/decimal"
)

var marketTokenPattern = regexp.MustCompile(`[A-Z0-9]+`)

type marketMetadata struct {
	BaseSymbol    string
	MarketAddr    string
	MarketName    string
	LotSize       decimal.Decimal
	MinSize       decimal.Decimal
	TickSize      decimal.Decimal
	PriceDecimals int32
	SizeDecimals  int32
}

func (m marketMetadata) EncodePrice(price decimal.Decimal) (uint64, error) {
	return m.encodePrice(price)
}

func (m marketMetadata) EncodeSize(size decimal.Decimal) (uint64, error) {
	return m.encodeSize(size)
}

type marketMetadataCache struct {
	bySymbol     map[string]marketMetadata
	byMarketAddr map[string]string
	byAlias      map[string]string
	sorted       []string
}

func newMarketMetadataCache(markets []decibelrest.Market) (*marketMetadataCache, error) {
	cache := &marketMetadataCache{
		bySymbol:     make(map[string]marketMetadata),
		byMarketAddr: make(map[string]string),
		byAlias:      make(map[string]string),
	}

	for _, market := range markets {
		if !isPerpMode(market.Mode) {
			continue
		}

		meta, err := newMarketMetadata(market)
		if err != nil {
			return nil, err
		}

		if existing, ok := cache.bySymbol[meta.BaseSymbol]; ok && existing.MarketAddr != meta.MarketAddr {
			return nil, fmt.Errorf(
				"duplicate base symbol %q for market addresses %s and %s",
				meta.BaseSymbol,
				existing.MarketAddr,
				meta.MarketAddr,
			)
		}
		if _, ok := cache.bySymbol[meta.BaseSymbol]; ok {
			continue
		}

		cache.bySymbol[meta.BaseSymbol] = meta
		cache.byMarketAddr[strings.ToLower(meta.MarketAddr)] = meta.BaseSymbol
		cache.byAlias[normalizeMarketAlias(meta.MarketName)] = meta.BaseSymbol
		cache.byAlias[normalizeMarketAlias(meta.BaseSymbol)] = meta.BaseSymbol
		cache.sorted = append(cache.sorted, meta.BaseSymbol)
	}

	sort.Strings(cache.sorted)
	return cache, nil
}

func newMarketMetadata(market decibelrest.Market) (marketMetadata, error) {
	baseSymbol, err := extractBaseSymbol(market.MarketName)
	if err != nil {
		return marketMetadata{}, err
	}

	return marketMetadata{
		BaseSymbol:    baseSymbol,
		MarketAddr:    market.MarketAddr,
		MarketName:    market.MarketName,
		LotSize:       normalizePrecisionValue(market.LotSize, market.SzDecimals),
		MinSize:       normalizePrecisionValue(market.MinSize, market.SzDecimals),
		TickSize:      normalizePrecisionValue(market.TickSize, market.PxDecimals),
		PriceDecimals: market.PxDecimals,
		SizeDecimals:  market.SzDecimals,
	}, nil
}

func normalizePrecisionValue(value decimal.Decimal, precision int32) decimal.Decimal {
	if precision <= 0 {
		return value
	}
	if !value.Equal(value.Truncate(0)) {
		return value
	}
	return value.Shift(-precision)
}

func extractBaseSymbol(name string) (string, error) {
	tokens := marketTokenPattern.FindAllString(strings.ToUpper(strings.TrimSpace(name)), -1)
	if len(tokens) == 0 {
		return "", fmt.Errorf("unable to extract base symbol from market name %q", name)
	}
	return tokens[0], nil
}

func isPerpMode(mode string) bool {
	normalized := strings.ToLower(strings.TrimSpace(mode))
	if normalized == "" {
		return true
	}
	if strings.Contains(normalized, "spot") {
		return false
	}
	return strings.Contains(normalized, "perp") ||
		normalized == "open" ||
		normalized == "reduceonly" ||
		normalized == "closeonly"
}

func normalizeMarketAlias(value string) string {
	tokens := marketTokenPattern.FindAllString(strings.ToUpper(strings.TrimSpace(value)), -1)
	return strings.Join(tokens, "-")
}

func (c *marketMetadataCache) symbols() []string {
	out := make([]string, len(c.sorted))
	copy(out, c.sorted)
	return out
}

func (c *marketMetadataCache) metadata(symbol string) (marketMetadata, error) {
	meta, ok := c.bySymbol[strings.ToUpper(strings.TrimSpace(symbol))]
	if !ok {
		return marketMetadata{}, exchanges.NewExchangeError(
			"DECIBEL",
			"",
			fmt.Sprintf("symbol not found: %s", symbol),
			exchanges.ErrSymbolNotFound,
		)
	}
	return meta, nil
}

func (c *marketMetadataCache) marketAddress(symbol string) (string, error) {
	meta, err := c.metadata(symbol)
	if err != nil {
		return "", err
	}
	return meta.MarketAddr, nil
}

func (c *marketMetadataCache) symbolForMarket(market string) (string, error) {
	if symbol, ok := c.byMarketAddr[strings.ToLower(strings.TrimSpace(market))]; ok {
		return symbol, nil
	}

	if symbol, ok := c.byAlias[normalizeMarketAlias(market)]; ok {
		return symbol, nil
	}

	return "", exchanges.NewExchangeError(
		"DECIBEL",
		"",
		fmt.Sprintf("market not found: %s", market),
		exchanges.ErrSymbolNotFound,
	)
}

func (c *marketMetadataCache) symbolDetails(symbol string) (*exchanges.SymbolDetails, error) {
	meta, err := c.metadata(symbol)
	if err != nil {
		return nil, err
	}
	return meta.symbolDetails(), nil
}

func symbolDetailsFromMetadataCache(cache *marketMetadataCache) map[string]*exchanges.SymbolDetails {
	if cache == nil {
		return nil
	}

	details := make(map[string]*exchanges.SymbolDetails, len(cache.bySymbol))
	for _, symbol := range cache.symbols() {
		detail, err := cache.symbolDetails(symbol)
		if err != nil {
			continue
		}
		details[symbol] = detail
	}
	return details
}

func (m marketMetadata) symbolDetails() *exchanges.SymbolDetails {
	return &exchanges.SymbolDetails{
		Symbol:            m.BaseSymbol,
		PricePrecision:    m.PriceDecimals,
		QuantityPrecision: m.SizeDecimals,
		MinQuantity:       m.MinSize,
	}
}

func (m marketMetadata) quantizePrice(price decimal.Decimal) (decimal.Decimal, error) {
	if price.IsZero() {
		return decimal.Zero, nil
	}
	if m.TickSize.IsPositive() {
		steps := price.Div(m.TickSize).Floor()
		price = steps.Mul(m.TickSize)
	}
	return price.Truncate(m.PriceDecimals), nil
}

func (m marketMetadata) quantizeSize(size decimal.Decimal) (decimal.Decimal, error) {
	size = size.Truncate(m.SizeDecimals)
	if m.LotSize.IsPositive() {
		steps := size.Div(m.LotSize).Floor()
		size = steps.Mul(m.LotSize).Truncate(m.SizeDecimals)
	}
	if m.MinSize.IsPositive() && size.LessThan(m.MinSize) {
		return decimal.Zero, exchanges.NewExchangeError(
			"DECIBEL",
			"",
			fmt.Sprintf("size %s below minimum %s", size, m.MinSize),
			exchanges.ErrMinQuantity,
		)
	}
	return size, nil
}

func (m marketMetadata) priceToChainUnits(price decimal.Decimal) (decimal.Decimal, error) {
	price, err := m.quantizePrice(price)
	if err != nil {
		return decimal.Zero, err
	}
	return scaleToUnits(price, m.PriceDecimals), nil
}

func (m marketMetadata) sizeToChainUnits(size decimal.Decimal) (decimal.Decimal, error) {
	size, err := m.quantizeSize(size)
	if err != nil {
		return decimal.Zero, err
	}
	return scaleToUnits(size, m.SizeDecimals), nil
}

func (m marketMetadata) encodePrice(price decimal.Decimal) (uint64, error) {
	return m.encodeChainValue(price, m.TickSize, m.PriceDecimals, decimal.Zero, exchanges.ErrInvalidPrecision)
}

func (m marketMetadata) encodeSize(size decimal.Decimal) (uint64, error) {
	return m.encodeChainValue(size, m.LotSize, m.SizeDecimals, m.MinSize, exchanges.ErrMinQuantity)
}

func (m marketMetadata) encodeChainValue(
	value decimal.Decimal,
	step decimal.Decimal,
	precision int32,
	min decimal.Decimal,
	minErr error,
) (uint64, error) {
	if value.IsNegative() {
		return 0, newPrecisionError("value must be non-negative")
	}

	if !value.Equal(value.Truncate(precision)) {
		return 0, newPrecisionError(
			fmt.Sprintf("value %s exceeds %d decimal places", value, precision),
		)
	}

	if min.IsPositive() && value.LessThan(min) {
		return 0, exchanges.NewExchangeError(
			"DECIBEL",
			"",
			fmt.Sprintf("value %s below minimum %s", value, min),
			minErr,
		)
	}

	if step.IsPositive() {
		steps := value.Div(step)
		if !steps.Equal(steps.Truncate(0)) {
			return 0, newPrecisionError(
				fmt.Sprintf("value %s is not a multiple of %s", value, step),
			)
		}
	}

	scaled := scaleToUnits(value, precision)
	if !scaled.Equal(scaled.Truncate(0)) {
		return 0, newPrecisionError(
			fmt.Sprintf("value %s cannot be represented in chain units", value),
		)
	}

	units, err := decimalToUint64(scaled)
	if err != nil {
		return 0, err
	}
	return units, nil
}

func scaleToUnits(value decimal.Decimal, precision int32) decimal.Decimal {
	return value.Mul(decimal.New(1, precision)).Truncate(0)
}

func decimalToUint64(value decimal.Decimal) (uint64, error) {
	if value.IsNegative() {
		return 0, newPrecisionError("value must be non-negative")
	}

	str := value.String()
	if strings.Contains(str, ".") {
		return 0, newPrecisionError(fmt.Sprintf("value %s is not an integer", value))
	}

	units, err := strconv.ParseUint(str, 10, 64)
	if err == nil {
		return units, nil
	}
	if errors.Is(err, strconv.ErrRange) {
		return 0, newPrecisionError(
			fmt.Sprintf("value %s exceeds max uint64 %d", value, uint64(math.MaxUint64)),
		)
	}
	return 0, newPrecisionError(fmt.Sprintf("invalid encoded value %s", value))
}

func newPrecisionError(message string) error {
	return exchanges.NewExchangeError("DECIBEL", "", message, exchanges.ErrInvalidPrecision)
}

func normalizeOrderStatus(status string) exchanges.OrderStatus {
	return decibelws.NormalizeOrderStatus(status)
}
