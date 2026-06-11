# Lighter Official API Parity

Source: https://apidocs.lighter.xyz/reference

Accessed: 2026-06-10

| Exchange | Product | Method | Path | Status | Local Symbol |
| --- | --- | --- | --- | --- | --- |
| LIGHTER | perp | GET | /api/v1/orderBooks | implemented-adapter | lighter/sdk.Client.GetOrderBooks; lighter.Adapter.FetchOrderBook |
| LIGHTER | spot | GET | /api/v1/orderBooks | implemented-adapter | lighter/sdk.Client.GetOrderBooks; lighter.SpotAdapter.FetchOrderBook |
| LIGHTER | perp | GET | /api/v1/recentTrades | implemented-adapter | lighter/sdk.Client.GetRecentTrades; lighter.Adapter.FetchTrades |
| LIGHTER | perp | GET | /api/v1/candlesticks | implemented-adapter | lighter/sdk.Client.GetCandlesticks; lighter.Adapter.FetchKlines |
| LIGHTER | perp | GET | /api/v1/fundingRates | implemented-adapter | lighter/sdk.Client.GetFundingRates; lighter.Adapter.FetchAllFundingRates |
| LIGHTER | perp | GET | /api/v1/fundingRate | implemented-adapter | lighter/sdk.Client.GetFundingRate; lighter.Adapter.FetchFundingRate |
| LIGHTER | perp | GET | /api/v1/account | implemented-adapter | lighter/sdk.Client.GetAccount; lighter.Adapter.FetchAccount |
| LIGHTER | account | GET | /api/v1/accountLimits | implemented-sdk | lighter/sdk.Client.GetAccountLimits; lighter/sdk.AccountLimitsResponse.AccountTier |
| LIGHTER | perp | GET | /api/v1/accountActiveOrders | implemented-adapter | lighter/sdk.Client.GetAccountActiveOrders; lighter.Adapter.FetchOpenOrders |
| LIGHTER | perp | POST | /api/v1/createOrder | implemented-adapter | lighter/sdk.Client.PlaceOrder; lighter.Adapter.PlaceOrder |
| LIGHTER | perp | POST | /api/v1/cancelOrder | implemented-adapter | lighter/sdk.Client.CancelOrder; lighter.Adapter.CancelOrder |
| LIGHTER | perp | POST | /api/v1/modifyOrder | implemented-adapter | lighter/sdk.Client.ModifyOrder; lighter.Adapter.ModifyOrder |
| LIGHTER | perp | POST | /api/v1/sendTxBatch | implemented-sdk | lighter/sdk.Client.SendTxBatch |
| LIGHTER | perp | GET | /api/v1/accountTxs | implemented-sdk | lighter/sdk.Client.GetAccountTxs |
| LIGHTER | perp | GET | /api/v1/positionFunding | implemented-sdk | lighter/sdk.Client.GetPositionFunding |
| LIGHTER | perp | POST | /api/v1/updateLeverage | implemented-adapter | lighter/sdk.Client.UpdateLeverage; lighter.Adapter.SetLeverage |
| LIGHTER | spot | WS | account_spot_avg_entry_prices/{account_id} | implemented-sdk | lighter/sdk.WebsocketClient.SubscribeAccountSpotAvgEntryPrices |
