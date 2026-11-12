package whatsmeow_service

import (
	"sync"
	"time"
)

const (
	defaultViewOnceCacheTTL        = 60 * time.Minute
	defaultViewOnceCacheMaxEntries = 200
	defaultViewOnceMaxItemBytes    = 16 * 1024 * 1024 // 16MB
)

// ViewOnceCachedMedia holds eagerly captured view-once media bytes in memory.
type ViewOnceCachedMedia struct {
	Data      []byte
	Mimetype  string
	Filename  string
	CreatedAt time.Time
}

type viewOnceCacheEntry struct {
	key   string
	value *ViewOnceCachedMedia
}

// ViewOnceCache is a bounded, TTL-based thread-safe cache for view-once media.
type ViewOnceCache struct {
	mu           sync.Mutex
	items        map[string]*viewOnceCacheEntry
	order        []string
	ttl          time.Duration
	maxEntries   int
	maxItemBytes int
}

// NewViewOnceCache creates a bounded view-once media cache.
func NewViewOnceCache(ttl time.Duration, maxEntries, maxItemBytes int) *ViewOnceCache {
	if ttl <= 0 {
		ttl = defaultViewOnceCacheTTL
	}
	if maxEntries <= 0 {
		maxEntries = defaultViewOnceCacheMaxEntries
	}
	if maxItemBytes <= 0 {
		maxItemBytes = defaultViewOnceMaxItemBytes
	}

	return &ViewOnceCache{
		items:        make(map[string]*viewOnceCacheEntry, maxEntries),
		order:        make([]string, 0, maxEntries),
		ttl:          ttl,
		maxEntries:   maxEntries,
		maxItemBytes: maxItemBytes,
	}
}

func makeViewOnceCacheKey(instanceID, msgID string) string {
	return instanceID + ":" + msgID
}

// Set stores a copy of data if within size limits, evicting expired/oldest entries as needed.
func (c *ViewOnceCache) Set(instanceID, msgID string, data []byte, mimetype, filename string) {
	if c == nil || instanceID == "" || msgID == "" || len(data) == 0 {
		return
	}
	if len(data) > c.maxItemBytes {
		return
	}

	now := time.Now()
	key := makeViewOnceCacheKey(instanceID, msgID)

	copied := make([]byte, len(data))
	copy(copied, data)

	entry := &viewOnceCacheEntry{
		key: key,
		value: &ViewOnceCachedMedia{
			Data:      copied,
			Mimetype:  mimetype,
			Filename:  filename,
			CreatedAt: now,
		},
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.pruneExpiredLocked(now)

	if existingIdx := c.findOrderIndexLocked(key); existingIdx != -1 {
		c.removeAtLocked(existingIdx)
	}

	for len(c.items) >= c.maxEntries && len(c.order) > 0 {
		c.removeAtLocked(0)
	}

	c.items[key] = entry
	c.order = append(c.order, key)
}

// Get returns cached view-once media if present and not expired.
func (c *ViewOnceCache) Get(instanceID, msgID string) (*ViewOnceCachedMedia, bool) {
	if c == nil || instanceID == "" || msgID == "" {
		return nil, false
	}

	now := time.Now()
	key := makeViewOnceCacheKey(instanceID, msgID)

	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.items[key]
	if !ok || entry == nil || entry.value == nil {
		return nil, false
	}

	if now.Sub(entry.value.CreatedAt) > c.ttl {
		if idx := c.findOrderIndexLocked(key); idx != -1 {
			c.removeAtLocked(idx)
		} else {
			delete(c.items, key)
		}
		return nil, false
	}

	copied := make([]byte, len(entry.value.Data))
	copy(copied, entry.value.Data)

	return &ViewOnceCachedMedia{
		Data:      copied,
		Mimetype:  entry.value.Mimetype,
		Filename:  entry.value.Filename,
		CreatedAt: entry.value.CreatedAt,
	}, true
}

func (c *ViewOnceCache) pruneExpiredLocked(now time.Time) {
	kept := c.order[:0]
	for _, key := range c.order {
		entry, ok := c.items[key]
		if !ok || entry == nil || entry.value == nil || now.Sub(entry.value.CreatedAt) > c.ttl {
			delete(c.items, key)
			continue
		}
		kept = append(kept, key)
	}
	c.order = kept
}

func (c *ViewOnceCache) findOrderIndexLocked(key string) int {
	for i, k := range c.order {
		if k == key {
			return i
		}
	}
	return -1
}

func (c *ViewOnceCache) removeAtLocked(idx int) {
	if idx < 0 || idx >= len(c.order) {
		return
	}
	key := c.order[idx]
	delete(c.items, key)
	c.order = append(c.order[:idx], c.order[idx+1:]...)
}
