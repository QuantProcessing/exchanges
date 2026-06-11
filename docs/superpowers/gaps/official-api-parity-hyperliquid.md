# Hyperliquid Official API Parity

Sources:

- https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/info-endpoint
- https://hyperliquid.gitbook.io/hyperliquid-docs/for-developers/api/exchange-endpoint

Accessed: 2026-06-10

| Exchange | Product | Method | Path | Status | Local Symbol |
| --- | --- | --- | --- | --- | --- |
| HYPERLIQUID | perp | POST | /info type=metaAndAssetCtxs | implemented-adapter | hyperliquid/sdk/perp.Client.GetMetaAndAssetCtxs; hyperliquid.Adapter.FetchSymbolDetails |
| HYPERLIQUID | perp | POST | /info type=allMids | implemented-adapter | hyperliquid/sdk/perp.Client.AllMids; hyperliquid.Adapter.FetchTicker |
| HYPERLIQUID | perp | POST | /info type=l2Book | implemented-adapter | hyperliquid/sdk/perp.Client.L2Book; hyperliquid.Adapter.FetchOrderBook |
| HYPERLIQUID | perp | POST | /info type=candleSnapshot | implemented-adapter | hyperliquid/sdk/perp.Client.CandleSnapshot; hyperliquid.Adapter.FetchKlines |
| HYPERLIQUID | perp | POST | /info type=userFills | implemented-sdk | hyperliquid/sdk/perp.Client.UserFills |
| HYPERLIQUID | perp | POST | /info type=clearinghouseState | implemented-adapter | hyperliquid/sdk/perp.Client.GetPerpPosition; hyperliquid.Adapter.FetchPositions |
| HYPERLIQUID | perp | POST | /exchange action=order | implemented-adapter | hyperliquid/sdk/perp.Client.PlaceOrder; hyperliquid.Adapter.PlaceOrder |
| HYPERLIQUID | perp | POST | /exchange action=cancel | implemented-adapter | hyperliquid/sdk/perp.Client.CancelOrder; hyperliquid.Adapter.CancelOrder |
| HYPERLIQUID | perp | POST | /exchange action=updateLeverage | implemented-adapter | hyperliquid/sdk/perp.Client.UpdateLeverage; hyperliquid.Adapter.SetLeverage |
| HYPERLIQUID | spot | POST | /info type=spotMeta | implemented-adapter | hyperliquid/sdk/spot.Client.GetSpotMeta; hyperliquid.SpotAdapter.FetchSymbolDetails |
| HYPERLIQUID | spot | POST | /info type=l2Book | implemented-adapter | hyperliquid/sdk/spot.Client.L2Book; hyperliquid.SpotAdapter.FetchOrderBook |
| HYPERLIQUID | spot | POST | /exchange action=order | implemented-adapter | hyperliquid/sdk/spot.Client.PlaceOrder; hyperliquid.SpotAdapter.PlaceOrder |
| HYPERLIQUID | perp | POST | /info type=frontendOpenOrders | implemented-raw | hyperliquid/sdk.Client.Post |
| HYPERLIQUID | perp | POST | /info type=userFillsByTime | implemented-raw | hyperliquid/sdk.Client.Post |
| HYPERLIQUID | perp | POST | /exchange action=scheduleCancel | implemented-raw | hyperliquid/sdk.Client.PostAction |
| HYPERLIQUID | spot | POST | /exchange action=usdClassTransfer | implemented-raw | hyperliquid/sdk.Client.PostAction |
| HYPERLIQUID | perp | POST | /exchange action=twapOrder | implemented-raw | hyperliquid/sdk.Client.PostAction |
