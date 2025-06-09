package cache

import (
	"sync"
	"time"
)

type QueryCache struct {
	mu         sync.RWMutex
	entries    map[string]*CacheEntry
	ttl        time.Duration
	maxEntries int
}

type CacheEntry struct {
	Result    []map[string]interface{}
	Timestamp time.Time
}

func NewQueryCache(ttl time.Duration, maxEntries int) *QueryCache {
	cache := &QueryCache{
		entries:    make(map[string]*CacheEntry),
		ttl:        ttl,
		maxEntries: maxEntries,
	}
	
	// Start cleanup goroutine
	go cache.cleanup()
	
	return cache
}

func (c *QueryCache) Get(query string) ([]map[string]interface{}, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	
	entry, exists := c.entries[query]
	if !exists || time.Since(entry.Timestamp) > c.ttl {
		return nil, false
	}
	
	return entry.Result, true
}

func (c *QueryCache) Set(query string, result []map[string]interface{}) {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	// Simple LRU: if at capacity, remove oldest entry
	if len(c.entries) >= c.maxEntries {
		var oldestKey string
		var oldestTime time.Time
		
		for k, v := range c.entries {
			if oldestTime.IsZero() || v.Timestamp.Before(oldestTime) {
				oldestKey = k
				oldestTime = v.Timestamp
			}
		}
		
		delete(c.entries, oldestKey)
	}
	
	c.entries[query] = &CacheEntry{
		Result:    result,
		Timestamp: time.Now(),
	}
}

func (c *QueryCache) cleanup() {
	ticker := time.NewTicker(c.ttl / 2)
	defer ticker.Stop()
	
	for range ticker.C {
		c.mu.Lock()
		now := time.Now()
		
		for key, entry := range c.entries {
			if now.Sub(entry.Timestamp) > c.ttl {
				delete(c.entries, key)
			}
		}
		
		c.mu.Unlock()
	}
}

func (c *QueryCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	
	c.entries = make(map[string]*CacheEntry)
}