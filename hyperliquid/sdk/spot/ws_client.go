package spot

import (
	"github.com/QuantProcessing/exchanges/hyperliquid/sdk"
)

type WebsocketClient struct {
	*hyperliquid.WebsocketClient
}

func NewWebsocketClient(base *hyperliquid.WebsocketClient) *WebsocketClient {
	return &WebsocketClient{WebsocketClient: base}
}
