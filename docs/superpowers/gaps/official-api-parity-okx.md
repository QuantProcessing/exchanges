# OKX Official API Parity

Source: https://www.okx.com/docs-v5/en/

Accessed: 2026-06-10

| Exchange | Product | Method | Path | Status | Local Symbol |
| --- | --- | --- | --- | --- | --- |
| OKX | spot | GET | /api/v5/market/ticker | implemented-adapter | okx/sdk.Client.GetTicker; okx.SpotAdapter.FetchTicker |
| OKX | swap | GET | /api/v5/market/ticker | implemented-adapter | okx/sdk.Client.GetTicker; okx.Adapter.FetchTicker |
| OKX | spot | GET | /api/v5/market/books | implemented-adapter | okx/sdk.Client.GetOrderBook; okx.SpotAdapter.FetchOrderBook |
| OKX | swap | GET | /api/v5/market/books | implemented-adapter | okx/sdk.Client.GetOrderBook; okx.Adapter.FetchOrderBook |
| OKX | spot | GET | /api/v5/market/candles | implemented-adapter | okx/sdk.Client.GetCandles; okx.SpotAdapter.FetchKlines |
| OKX | swap | GET | /api/v5/market/history-trades | implemented-adapter | okx/sdk.Client.GetHistoryTrades; okx.Adapter.FetchHistoricalTrades |
| OKX | spot | GET | /api/v5/public/instruments | implemented-adapter | okx/sdk.Client.GetInstruments; okx.SpotAdapter.FetchSymbolDetails |
| OKX | swap | GET | /api/v5/public/instruments | implemented-adapter | okx/sdk.Client.GetInstruments; okx.Adapter.FetchSymbolDetails |
| OKX | spot | GET | /api/v5/account/balance | implemented-adapter | okx/sdk.Client.GetAccountBalance; okx.SpotAdapter.FetchSpotBalances |
| OKX | swap | GET | /api/v5/account/positions | implemented-adapter | okx/sdk.Client.GetPositions; okx.Adapter.FetchPositions |
| OKX | account | GET | /api/v5/account/config | implemented-sdk | okx/sdk.Client.GetAccountConfig; okx/sdk.AccountConfig.AccountLevel |
| OKX | swap | POST | /api/v5/account/set-leverage | implemented-adapter | okx/sdk.Client.SetLeverage; okx.Adapter.SetLeverage |
| OKX | spot | GET | /api/v5/account/trade-fee | implemented-adapter | okx/sdk.Client.GetTradeFee; okx.SpotAdapter.FetchFeeRate |
| OKX | swap | GET | /api/v5/account/trade-fee | implemented-adapter | okx/sdk.Client.GetTradeFee; okx.Adapter.FetchFeeRate |
| OKX | spot | POST | /api/v5/trade/order | implemented-adapter | okx/sdk.Client.PlaceOrder; okx.SpotAdapter.PlaceOrder |
| OKX | swap | POST | /api/v5/trade/order | implemented-adapter | okx/sdk.Client.PlaceOrder; okx.Adapter.PlaceOrder |
| OKX | spot | POST | /api/v5/trade/cancel-order | implemented-adapter | okx/sdk.Client.CancelOrder; okx.SpotAdapter.CancelOrder |
| OKX | swap | POST | /api/v5/trade/amend-order | implemented-adapter | okx/sdk.Client.ModifyOrder; okx.Adapter.ModifyOrder |
| OKX | spot | GET | /api/v5/trade/order | implemented-adapter | okx/sdk.Client.GetOrder; okx.SpotAdapter.FetchOrderByID |
| OKX | swap | GET | /api/v5/trade/orders-pending | implemented-adapter | okx/sdk.Client.GetOrders; okx.Adapter.FetchOpenOrders |
| OKX | swap | POST | /api/v5/trade/batch-orders | implemented-raw | okx/sdk.Client.Do |
| OKX | swap | POST | /api/v5/trade/order-algo | implemented-raw | okx/sdk.Client.Do |
| OKX | spot | GET | /api/v5/account/bills | implemented-raw | okx/sdk.Client.Do |
