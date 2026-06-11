# Binance Official API Parity

Sources:

- Spot REST: https://developers.binance.com/docs/binance-spot-api-docs/rest-api
- USD-M Futures: https://developers.binance.com/docs/derivatives/usds-margined-futures/general-info
- Margin Account: https://developers.binance.com/docs/margin_trading/account
- Portfolio Margin Account: https://developers.binance.com/docs/derivatives/portfolio-margin/account
- Sub-Account Asset Management: https://developers.binance.com/docs/sub_account/asset-management

Accessed: 2026-06-11

| Exchange | Product | Method | Path | Status | Local Symbol |
| --- | --- | --- | --- | --- | --- |
| BINANCE | spot | GET | /api/v3/depth | implemented-adapter | binance/sdk/spot.Client.Depth; binance.SpotAdapter.FetchOrderBook |
| BINANCE | spot | GET | /api/v3/klines | implemented-adapter | binance/sdk/spot.Client.Klines; binance.SpotAdapter.FetchKlines |
| BINANCE | spot | GET | /api/v3/ticker/24hr | implemented-adapter | binance/sdk/spot.Client.Ticker; binance.SpotAdapter.FetchTicker |
| BINANCE | spot | GET | /api/v3/ticker/bookTicker | implemented-sdk | binance/sdk/spot.Client.BookTicker |
| BINANCE | spot | GET | /api/v3/exchangeInfo | implemented-adapter | binance/sdk/spot.Client.ExchangeInfo; binance.SpotAdapter.FetchSymbolDetails |
| BINANCE | spot | GET | /api/v3/account | implemented-adapter | binance/sdk/spot.Client.GetAccount; binance.SpotAdapter.FetchAccount |
| BINANCE | spot | POST | /api/v3/userDataStream | implemented-sdk | binance/sdk/spot.Client.StartUserDataStream |
| BINANCE | spot | PUT | /api/v3/userDataStream | implemented-sdk | binance/sdk/spot.Client.KeepAliveUserDataStream |
| BINANCE | spot | DELETE | /api/v3/userDataStream | implemented-sdk | binance/sdk/spot.Client.CloseUserDataStream |
| BINANCE | spot | POST | /api/v3/order | implemented-adapter | binance/sdk/spot.Client.PlaceOrder; binance.SpotAdapter.PlaceOrder |
| BINANCE | spot | DELETE | /api/v3/order | implemented-adapter | binance/sdk/spot.Client.CancelOrder; binance.SpotAdapter.CancelOrder |
| BINANCE | spot | POST | /api/v3/order/cancelReplace | implemented-adapter | binance/sdk/spot.Client.ModifyOrder; binance.SpotAdapter.ModifyOrder |
| BINANCE | spot | GET | /api/v3/order | implemented-adapter | binance/sdk/spot.Client.GetOrder; binance.SpotAdapter.FetchOrderByID |
| BINANCE | spot | GET | /api/v3/openOrders | implemented-adapter | binance/sdk/spot.Client.GetOpenOrders; binance.SpotAdapter.FetchOpenOrders |
| BINANCE | spot | GET | /api/v3/allOrders | implemented-raw | binance/sdk/spot.Client.Get |
| BINANCE | spot | POST | /api/v3/order/oco | intentionally-unsupported |  |
| BINANCE | spot | POST | /api/v3/orderList/oto | intentionally-unsupported |  |
| BINANCE | spot | POST | /api/v3/orderList/otoco | intentionally-unsupported |  |
| BINANCE | usd-m | GET | /fapi/v1/depth | implemented-adapter | binance/sdk/perp.Client.Depth; binance.Adapter.FetchOrderBook |
| BINANCE | usd-m | GET | /fapi/v1/klines | implemented-adapter | binance/sdk/perp.Client.Klines; binance.Adapter.FetchKlines |
| BINANCE | usd-m | GET | /fapi/v1/ticker/24hr | implemented-adapter | binance/sdk/perp.Client.Ticker; binance.Adapter.FetchTicker |
| BINANCE | usd-m | GET | /fapi/v1/exchangeInfo | implemented-adapter | binance/sdk/perp.Client.ExchangeInfo; binance.Adapter.FetchSymbolDetails |
| BINANCE | usd-m | GET | /fapi/v1/aggTrades | implemented-adapter | binance/sdk/perp.Client.GetAggTrades; binance.Adapter.FetchTrades |
| BINANCE | usd-m | GET | /fapi/v1/fundingRate | implemented-adapter | binance/sdk/perp.Client.GetFundingRateHistory; binance.Adapter.FetchFundingRateHistory |
| BINANCE | usd-m | GET | /fapi/v1/openInterest | implemented-adapter | binance/sdk/perp.Client.GetOpenInterest; binance.Adapter.FetchOpenInterest |
| BINANCE | usd-m | GET | /fapi/v2/account | implemented-adapter | binance/sdk/perp.Client.GetAccount; binance.Adapter.FetchAccount |
| BINANCE | usd-m | POST | /fapi/v1/order | implemented-adapter | binance/sdk/perp.Client.PlaceOrder; binance.Adapter.PlaceOrder |
| BINANCE | usd-m | DELETE | /fapi/v1/order | implemented-adapter | binance/sdk/perp.Client.CancelOrder; binance.Adapter.CancelOrder |
| BINANCE | usd-m | PUT | /fapi/v1/order | implemented-adapter | binance/sdk/perp.Client.ModifyOrder; binance.Adapter.ModifyOrder |
| BINANCE | usd-m | GET | /fapi/v1/order | implemented-adapter | binance/sdk/perp.Client.GetOrder; binance.Adapter.FetchOrderByID |
| BINANCE | usd-m | GET | /fapi/v1/openOrders | implemented-adapter | binance/sdk/perp.Client.GetOpenOrders; binance.Adapter.FetchOpenOrders |
| BINANCE | usd-m | POST | /fapi/v1/batchOrders | implemented-raw | binance/sdk/perp.Client.Post |
| BINANCE | usd-m | PUT | /fapi/v1/batchOrders | implemented-raw | binance/sdk/perp.Client.Put |
| BINANCE | usd-m | DELETE | /fapi/v1/batchOrders | implemented-raw | binance/sdk/perp.Client.Delete |
| BINANCE | margin | GET | /sapi/v1/margin/account | planned-sdk | SDK-first account product; do not add to core adapter |
| BINANCE | portfolio-margin | GET | /papi/v1/balance | implemented-sdk | binance/sdk/portfolio.Client.GetBalances |
| BINANCE | portfolio-margin | GET | /papi/v1/account | implemented-sdk | binance/sdk/portfolio.Client.GetAccount |
| BINANCE | sub-account | GET | /sapi/v4/sub-account/assets | implemented-sdk | binance/sdk/subaccount.Client.GetAssetsV4 |
| BINANCE | sub-account | GET | /sapi/v1/sub-account/spotSummary | implemented-sdk | binance/sdk/subaccount.Client.GetSpotAssetsSummary |
| BINANCE | sub-account | POST | /sapi/v1/sub-account/futures/transfer | implemented-sdk | binance/sdk/subaccount.Client.FuturesTransfer |
| BINANCE | sub-account | POST | /sapi/v1/sub-account/universalTransfer | implemented-sdk | binance/sdk/subaccount.Client.UniversalTransfer |
| BINANCE | coin-m | GET | /dapi/v1/exchangeInfo | intentionally-unsupported |  |
