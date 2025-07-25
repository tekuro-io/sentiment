package sentiment

import (
	"log"
	"sync"
	"time"
)

type sentimentCacheItem struct {
	value      *SentimentResponse
	expiration time.Time
	wait       chan struct{}
}

type SentimentCache struct {
	cache sync.Map
	ttl   time.Duration
}

func NewSentimentCache(ttl time.Duration) *SentimentCache {
	return &SentimentCache{
		ttl: ttl,
	}
}

func (c *SentimentCache) GetOrLoad(ticker string, fetch func() (*SentimentResponse, error)) (*SentimentResponse, error) {
	now := time.Now()

	if val, ok := c.cache.Load(ticker); ok {
		entry := val.(*sentimentCacheItem)
		if now.Before(entry.expiration) {
			<-entry.wait
			return entry.value, nil
		}
		c.cache.Delete(ticker)
	}

	entry := &sentimentCacheItem{
		wait: make(chan struct{}),
	}

	actual, loaded := c.cache.LoadOrStore(ticker, entry)
	if loaded {
		entry = actual.(*sentimentCacheItem)
		<-entry.wait
		return entry.value, nil
	}

	defer close(entry.wait)

	val, err := fetch()
	if err != nil {
		c.cache.Delete(ticker)
		log.Printf("Ticker failed to load: %v\n", err)
		return nil, err
	}

	entry.value = val
	entry.expiration = time.Now().Add(c.ttl)

	return val, nil
}
