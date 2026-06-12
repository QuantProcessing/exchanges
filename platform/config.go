package platform

import sharedcache "github.com/QuantProcessing/exchanges/cache"

type Config struct {
	Cache *sharedcache.Cache
	Bus   *Bus
}
