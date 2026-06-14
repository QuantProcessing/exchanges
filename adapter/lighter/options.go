package lighter

import "github.com/QuantProcessing/exchanges/model"

const Venue = model.Venue("LIGHTER")

type Options struct {
	PrivateKey   string
	AccountIndex int64
	KeyIndex     uint8
	AccountID    model.AccountID
}
