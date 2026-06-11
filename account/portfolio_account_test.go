package account_test

import (
	"context"
	"testing"

	exchanges "github.com/QuantProcessing/exchanges"
	"github.com/QuantProcessing/exchanges/account"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
)

func TestPortfolioAccountAggregatesRegisteredAccounts(t *testing.T) {
	t.Parallel()

	perpAdp := &accountRuntimeStubExchange{
		fetchAccountResp: &exchanges.Account{
			Orders: []exchanges.Order{{
				OrderID: "perp-order",
				Symbol:  "BTC/USDT",
				Status:  exchanges.OrderStatusNew,
			}},
			Positions: []exchanges.Position{{
				Symbol:   "BTC/USDT",
				Quantity: decimal.RequireFromString("0.2"),
			}},
			TotalBalance: decimal.RequireFromString("100"),
		},
	}
	perpAcct := account.NewPerpTradingAccount(perpAdp, nil)
	require.NoError(t, perpAcct.Start(context.Background()))
	defer perpAcct.Close()

	spotAdp := &spotAccountRuntimeStubExchange{
		accountRuntimeStubExchange: accountRuntimeStubExchange{
			fetchAccountResp: &exchanges.Account{
				Orders: []exchanges.Order{{
					OrderID: "spot-order",
					Symbol:  "ETH/USDC",
					Status:  exchanges.OrderStatusNew,
				}},
			},
		},
		spotBalances: []exchanges.SpotBalance{{
			Asset: "USDC",
			Total: decimal.RequireFromString("50"),
			Free:  decimal.RequireFromString("50"),
		}},
	}
	spotAcct := account.NewSpotTradingAccount(spotAdp, nil)
	require.NoError(t, spotAcct.Start(context.Background()))
	defer spotAcct.Close()

	portfolio := account.NewPortfolioAccount()
	perpKey := account.PortfolioAccountKey{
		Exchange:   "STUB",
		MarketType: exchanges.MarketTypePerp,
		Quote:      exchanges.QuoteCurrencyUSDT,
	}
	spotKey := account.PortfolioAccountKey{
		Exchange:   "STUB",
		MarketType: exchanges.MarketTypeSpot,
		Quote:      exchanges.QuoteCurrencyUSDC,
	}

	require.NoError(t, portfolio.AddPerp(perpKey, perpAcct))
	require.NoError(t, portfolio.AddSpot(spotKey, spotAcct))

	require.Len(t, portfolio.Keys(), 2)

	positions := portfolio.Positions()
	require.Len(t, positions, 1)
	require.Equal(t, perpKey, positions[0].Key)
	require.Equal(t, "BTC/USDT", positions[0].Position.Symbol)

	balances := portfolio.Balances()
	require.Len(t, balances, 1)
	require.Equal(t, spotKey, balances[0].Key)
	require.Equal(t, "USDC", balances[0].Balance.Asset)

	orders := portfolio.OpenOrders()
	require.Len(t, orders, 2)
	require.Equal(t, "perp-order", orders[0].Order.OrderID)
	require.Equal(t, "spot-order", orders[1].Order.OrderID)

	health := portfolio.Health()
	require.Len(t, health, 2)
	require.True(t, health[perpKey].Started)
	require.True(t, health[spotKey].Started)
}

func TestPortfolioAccountRejectsDuplicateKeysAndNilAccounts(t *testing.T) {
	t.Parallel()

	portfolio := account.NewPortfolioAccount()
	key := account.PortfolioAccountKey{
		Exchange:   "STUB",
		MarketType: exchanges.MarketTypePerp,
		Quote:      exchanges.QuoteCurrencyUSDT,
	}

	require.Error(t, portfolio.AddPerp(key, nil))

	acct := account.NewPerpTradingAccount(&accountRuntimeStubExchange{}, nil)
	require.NoError(t, portfolio.AddPerp(key, acct))
	require.Error(t, portfolio.AddPerp(key, acct))
	require.Error(t, portfolio.AddSpot(key, nil))
}
