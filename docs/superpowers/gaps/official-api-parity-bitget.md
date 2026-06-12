# Bitget Official API Parity

Sources:
- https://www.bitget.com/api-doc/common/intro
- https://www.bitget.com/api-doc/uta/intro
- https://www.bitget.com/api-doc/uta/trade/Place-Order
- https://www.bitget.com/api-doc/uta/account/Get-Account-Funding-Assets
- https://www.bitget.com/api-doc/uta/account/Get-Account-Info
- https://www.bitget.com/api-doc/uta/account/Get-Financial-Records
- https://www.bitget.com/api-doc/uta/account/Get-Account-Fee-Rate
- https://www.bitget.com/api-doc/uta/account/Get-Switch-Status
- https://www.bitget.com/api-doc/uta/account/Get-Max-Transferable
- https://www.bitget.com/api-doc/uta/account/Get-OI-Limit
- https://www.bitget.com/api-doc/uta/websocket/private/Order-Channel
- https://www.bitget.com/api-doc/uta/websocket/private/Fill-Channel

Accessed: 2026-06-11

| Exchange | Product | Method | Path | Status | Local Symbol |
| --- | --- | --- | --- | --- | --- |
| BITGET | spot | GET | /api/v2/spot/public/symbols | implemented-adapter | bitget/sdk.Client.GetInstruments; bitget.SpotAdapter.FetchSymbolDetails |
| BITGET | futures | GET | /api/v2/mix/market/contracts | implemented-adapter | bitget/sdk.Client.GetInstruments; bitget.Adapter.FetchSymbolDetails |
| BITGET | spot | GET | /api/v2/spot/market/tickers | implemented-adapter | bitget/sdk.Client.GetTicker; bitget.SpotAdapter.FetchTicker |
| BITGET | futures | GET | /api/v2/mix/market/ticker | implemented-adapter | bitget/sdk.Client.GetTicker; bitget.Adapter.FetchTicker |
| BITGET | spot | GET | /api/v2/spot/market/orderbook | implemented-adapter | bitget/sdk.Client.GetOrderBook; bitget.SpotAdapter.FetchOrderBook |
| BITGET | futures | GET | /api/v2/mix/market/fills | implemented-adapter | bitget/sdk.Client.GetRecentFills; bitget.Adapter.FetchTrades |
| BITGET | futures | GET | /api/v2/mix/market/open-interest | implemented-adapter | bitget/sdk.Client.GetOpenInterest; bitget.Adapter.FetchOpenInterest |
| BITGET | futures | GET | /api/v2/mix/market/history-fund-rate | implemented-adapter | bitget/sdk.Client.GetHistoryFundRate; bitget.Adapter.FetchFundingRateHistory |
| BITGET | spot | POST | /api/v2/spot/trade/place-order | implemented-adapter | bitget/sdk.Client.PlaceOrder; bitget.SpotAdapter.PlaceOrder |
| BITGET | futures | POST | /api/v2/mix/order/place-order | implemented-adapter | bitget/sdk.Client.PlaceOrder; bitget.Adapter.PlaceOrder |
| BITGET | spot | POST | /api/v2/spot/trade/cancel-order | implemented-adapter | bitget/sdk.Client.CancelOrder; bitget.SpotAdapter.CancelOrder |
| BITGET | futures | POST | /api/v2/mix/order/modify-order | implemented-adapter | bitget/sdk.Client.ModifyOrder; bitget.Adapter.ModifyOrder |
| BITGET | futures | GET | /api/v2/mix/order/orders-pending | implemented-adapter | bitget/sdk.Client.GetOpenOrders; bitget.Adapter.FetchOpenOrders |
| BITGET | spot | POST | /api/v2/spot/trade/batch-orders | implemented-raw | bitget/sdk.Client.PostPrivateRaw |
| BITGET | futures | POST | /api/v2/mix/order/place-plan-order | implemented-raw | bitget/sdk.Client.PostPrivateRaw |
| BITGET | futures | POST | /api/v2/mix/account/set-margin-mode | implemented-raw | bitget/sdk.Client.PostPrivateRaw |
| BITGET | futures | GET | /api/v2/mix/position/history-position | implemented-raw | bitget/sdk.Client.GetPrivateRaw |
| BITGET | uta spot/futures | POST | /api/v3/trade/place-order | implemented-adapter | bitget/sdk.Client.PlaceOrder; bitget.utaPerpProfile.PlaceOrder; bitget.utaSpotProfile.PlaceOrder |
| BITGET | uta spot/futures | POST | /api/v3/trade/cancel-order | implemented-adapter | bitget/sdk.Client.CancelOrder; bitget.utaPerpProfile.CancelOrder; bitget.utaSpotProfile.CancelOrder |
| BITGET | uta spot/futures | POST | /api/v3/trade/cancel-symbol-order | implemented-adapter | bitget/sdk.Client.CancelAllOrders; bitget.utaPerpProfile.CancelAllOrders; bitget.utaSpotProfile.CancelAllOrders |
| BITGET | uta futures | POST | /api/v3/trade/modify-order | implemented-adapter | bitget/sdk.Client.ModifyOrder; bitget.utaPerpProfile.ModifyOrder |
| BITGET | uta spot/futures | GET | /api/v3/trade/order-info | implemented-adapter | bitget/sdk.Client.GetOrder; bitget.utaPerpProfile.FetchOrderByID; bitget.utaSpotProfile.FetchOrderByID |
| BITGET | uta spot/futures | GET | /api/v3/trade/unfilled-orders | implemented-adapter | bitget/sdk.Client.GetOpenOrders; bitget.utaPerpProfile.FetchOpenOrders; bitget.utaSpotProfile.FetchOpenOrders |
| BITGET | uta spot/futures | GET | /api/v3/trade/history-orders | implemented-adapter | bitget/sdk.Client.GetOrderHistory; bitget.utaPerpProfile.FetchOrders; bitget.utaSpotProfile.FetchOrders |
| BITGET | uta spot/futures | GET | /api/v3/account/assets | implemented-adapter | bitget/sdk.Client.GetAccountAssets; bitget.utaPerpProfile.FetchAccount; bitget.utaSpotProfile.FetchAccount |
| BITGET | uta account | GET | /api/v3/account/info | implemented-sdk | bitget/sdk.Client.GetAccountInfo |
| BITGET | uta account | GET | /api/v3/account/funding-assets | implemented-sdk | bitget/sdk.Client.GetFundingAssets |
| BITGET | uta account | GET | /api/v3/account/financial-records | implemented-sdk | bitget/sdk.Client.GetFinancialRecords |
| BITGET | uta account | GET | /api/v3/account/fee-rate | implemented-sdk | bitget/sdk.Client.GetAccountFeeRate |
| BITGET | uta account | GET | /api/v3/account/switch-status | implemented-sdk | bitget/sdk.Client.GetSwitchStatus |
| BITGET | uta account | GET | /api/v3/account/max-transferable | implemented-sdk | bitget/sdk.Client.GetMaxTransferable |
| BITGET | uta futures | GET | /api/v3/account/open-interest-limit | implemented-sdk | bitget/sdk.Client.GetOpenInterestLimit |
| BITGET | uta account | POST | /api/v3/account/set-hold-mode | implemented-sdk | bitget/sdk.Client.SetHoldMode |
| BITGET | uta futures | GET | /api/v3/position/current-position | implemented-adapter | bitget/sdk.Client.GetCurrentPositions; bitget.utaPerpProfile.FetchPositions |
| BITGET | uta futures | POST | /api/v3/account/set-leverage | implemented-adapter | bitget/sdk.Client.SetLeverage; bitget.utaPerpProfile.SetLeverage |
| BITGET | uta spot/futures | WS trade | topic=place-order | implemented-adapter | bitget/sdk.PrivateWSClient.PlaceUTAOrderWS; bitget.utaPerpProfile.PlaceOrderWS; bitget.utaSpotProfile.PlaceOrderWS |
| BITGET | uta spot/futures | WS trade | topic=cancel-order | implemented-adapter | bitget/sdk.PrivateWSClient.CancelUTAOrderWS; bitget.utaPerpProfile.CancelOrderWS; bitget.utaSpotProfile.CancelOrderWS |
| BITGET | uta spot/futures | WS private | topic=order | implemented-adapter | bitget/sdk.DecodeOrderMessage; bitget.utaPerpProfile.WatchOrders; bitget.utaSpotProfile.WatchOrders |
| BITGET | uta spot/futures | WS private | topic=fill | implemented-adapter | bitget/sdk.DecodeFillMessage; bitget.utaPerpProfile.WatchFills; bitget.utaSpotProfile.WatchFills |
| BITGET | uta futures | WS private | topic=position | implemented-adapter | bitget/sdk.DecodePositionMessage; bitget.utaPerpProfile.WatchPositions |

Notes:
- UTA private streams use `instType=UTA` plus `topic=order/fill/position`; classic private streams continue to use the v2 `channel` shape.
- UTA live private read tests currently skip with classic-account credentials (`40084`). A real UTA-enabled key is required to live-verify private UTA REST and private WS subscription success.
