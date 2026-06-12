# Bybit Official API Parity

Source: https://bybit-exchange.github.io/docs/v5/intro

Accessed: 2026-06-10

| Exchange | Product | Method | Path | Status | Local Symbol |
| --- | --- | --- | --- | --- | --- |
| BYBIT | spot | GET | /v5/market/instruments-info | implemented-adapter | bybit/sdk.Client.GetInstruments; bybit.SpotAdapter.FetchSymbolDetails |
| BYBIT | linear | GET | /v5/market/instruments-info | implemented-adapter | bybit/sdk.Client.GetInstruments; bybit.Adapter.FetchSymbolDetails |
| BYBIT | spot | GET | /v5/market/tickers | implemented-adapter | bybit/sdk.Client.GetTicker; bybit.SpotAdapter.FetchTicker |
| BYBIT | linear | GET | /v5/market/tickers | implemented-adapter | bybit/sdk.Client.GetTicker; bybit.Adapter.FetchTicker |
| BYBIT | spot | GET | /v5/market/orderbook | implemented-adapter | bybit/sdk.Client.GetOrderBook; bybit.SpotAdapter.FetchOrderBook |
| BYBIT | linear | GET | /v5/market/recent-trade | implemented-adapter | bybit/sdk.Client.GetRecentTrades; bybit.Adapter.FetchTrades |
| BYBIT | linear | GET | /v5/market/open-interest | implemented-adapter | bybit/sdk.Client.GetOpenInterest; bybit.Adapter.FetchOpenInterest |
| BYBIT | linear | GET | /v5/market/funding/history | implemented-adapter | bybit/sdk.Client.GetFundingHistory; bybit.Adapter.FetchFundingRateHistory |
| BYBIT | account | GET | /v5/account/info | implemented-sdk | bybit/sdk.Client.GetAccountInfo |
| BYBIT | spot | POST | /v5/order/create | implemented-adapter | bybit/sdk.Client.PlaceOrder; bybit.SpotAdapter.PlaceOrder |
| BYBIT | linear | POST | /v5/order/create | implemented-adapter | bybit/sdk.Client.PlaceOrder; bybit.Adapter.PlaceOrder |
| BYBIT | spot | POST | /v5/order/cancel | implemented-adapter | bybit/sdk.Client.CancelOrder; bybit.SpotAdapter.CancelOrder |
| BYBIT | linear | POST | /v5/order/amend | implemented-adapter | bybit/sdk.Client.AmendOrder; bybit.Adapter.ModifyOrder |
| BYBIT | linear | GET | /v5/order/realtime | implemented-adapter | bybit/sdk.Client.GetOpenOrders; bybit.Adapter.FetchOpenOrders |
| BYBIT | linear | POST | /v5/order/create-batch | implemented-raw | bybit/sdk.Client.PostPrivateRaw |
| BYBIT | linear | POST | /v5/order/amend-batch | implemented-raw | bybit/sdk.Client.PostPrivateRaw |
| BYBIT | linear | POST | /v5/order/cancel-batch | implemented-raw | bybit/sdk.Client.PostPrivateRaw |
| BYBIT | linear | POST | /v5/position/trading-stop | implemented-raw | bybit/sdk.Client.PostPrivateRaw |
| BYBIT | linear | GET | /v5/position/closed-pnl | implemented-raw | bybit/sdk.Client.GetPrivateRaw |
