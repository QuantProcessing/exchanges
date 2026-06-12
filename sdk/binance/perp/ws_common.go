package perp

const (
	WSBaseURL    = "wss://fstream.binance.com/ws"
	WSAPIBaseURL = "wss://ws-fapi.binance.com/ws-fapi/v1"
)

type MsgDispatcher interface {
	Dispatch(data []byte) error
}
