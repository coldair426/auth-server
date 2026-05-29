package cache

import (
	"context"
	"sync"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/google/uuid"
)

// clientSource is the underlying source ClientCache falls back to on a miss.
// Satisfied by *postgres.OAuthClientRepo (and any stub in tests).
type clientSource interface {
	FindByID(ctx context.Context, id uuid.UUID) (domain.OAuthClient, error)
}

// ClientCache is a read-through, write-invalidate in-memory cache for
// OAuthClient records. It is safe for concurrent use.
//
// Cold start: an empty cache falls through transparently to the source.
// Invalidate must be called whenever the underlying record is mutated.
type ClientCache struct {
	mu     sync.RWMutex
	data   map[uuid.UUID]domain.OAuthClient
	source clientSource
}

func NewClientCache(source clientSource) *ClientCache {
	return &ClientCache{
		data:   make(map[uuid.UUID]domain.OAuthClient),
		source: source,
	}
}

// Get returns the OAuthClient for id. On a cache miss it fetches from the
// source, populates the cache, and returns the result.
func (c *ClientCache) Get(ctx context.Context, id uuid.UUID) (domain.OAuthClient, error) {
	c.mu.RLock()
	if v, ok := c.data[id]; ok {
		c.mu.RUnlock()
		return v, nil
	}
	c.mu.RUnlock()

	client, err := c.source.FindByID(ctx, id)
	if err != nil {
		return domain.OAuthClient{}, err
	}

	c.mu.Lock()
	// double-check: another goroutine may have populated between RUnlock and Lock
	if _, ok := c.data[id]; !ok {
		c.data[id] = client
	}
	c.mu.Unlock()
	return client, nil
}

// Invalidate removes id from the cache. Safe to call even if id is absent.
func (c *ClientCache) Invalidate(id uuid.UUID) {
	c.mu.Lock()
	delete(c.data, id)
	c.mu.Unlock()
}

// FindByID satisfies auth.OAuthClientRepository and client.ClientRepository,
// allowing ClientCache to be used as a drop-in replacement for the raw repo.
func (c *ClientCache) FindByID(ctx context.Context, id uuid.UUID) (domain.OAuthClient, error) {
	return c.Get(ctx, id)
}
