package account

import "github.com/QuantProcessing/exchanges/cache"

type Cache = cache.Cache

func NewCache() *cache.Cache {
	return cache.New()
}
