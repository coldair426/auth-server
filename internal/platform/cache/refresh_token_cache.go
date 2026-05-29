package cache

import (
	"context"
	"sync"

	"github.com/coldair426/auth-server/internal/domain"
)

// tokenSource is the underlying Postgres-backed store.
// Satisfied by *postgres.RefreshTokenRepo.
type tokenSource interface {
	Create(ctx context.Context, t domain.RefreshToken) (domain.RefreshToken, error)
	FindByHash(ctx context.Context, hash string) (domain.RefreshToken, error)
	RevokeByHash(ctx context.Context, hash string) error
}

// RefreshTokenCache is an OPTIONAL read-through accelerator over a token source.
//
// Security contract (enforced by this layer):
//
//   - FindByHash: serves the cached token if present. Only active (non-revoked,
//     non-expired) tokens are stored in the cache. The SERVICE layer remains
//     responsible for treating the result as authoritative for revocation status
//     in high-security paths (e.g. token rotation) — it should call RevokeByHash
//     rather than relying on the cached IsRevoked() field alone.
//
//   - RevokeByHash: writes to Postgres FIRST. The cache entry is evicted only
//     after the Postgres write succeeds. A failed Postgres write leaves the cache
//     intact so subsequent reads fall through to Postgres and get the true state.
//
//   - Create: write-through to Postgres; the result is not pre-populated in the
//     cache (it will be cached on first FindByHash if still active at that point).
//
// Cold start: empty cache falls through transparently to the source.
type RefreshTokenCache struct {
	mu     sync.RWMutex
	data   map[string]domain.RefreshToken
	source tokenSource
}

func NewRefreshTokenCache(source tokenSource) *RefreshTokenCache {
	return &RefreshTokenCache{
		data:   make(map[string]domain.RefreshToken),
		source: source,
	}
}

// FindByHash returns the token from cache when it is present and still active.
// On a miss it delegates to the source and caches the result if active.
func (c *RefreshTokenCache) FindByHash(ctx context.Context, hash string) (domain.RefreshToken, error) {
	c.mu.RLock()
	if t, ok := c.data[hash]; ok {
		c.mu.RUnlock()
		return t, nil
	}
	c.mu.RUnlock()

	t, err := c.source.FindByHash(ctx, hash)
	if err != nil {
		return domain.RefreshToken{}, err
	}

	// only cache tokens that are still usable; revoked/expired tokens must always
	// be fetched from Postgres so revocation is observed immediately.
	if !t.IsRevoked() && !t.IsExpired() {
		c.mu.Lock()
		if _, ok := c.data[hash]; !ok {
			c.data[hash] = t
		}
		c.mu.Unlock()
	}
	return t, nil
}

// Create writes through to the underlying Postgres source.
func (c *RefreshTokenCache) Create(ctx context.Context, t domain.RefreshToken) (domain.RefreshToken, error) {
	return c.source.Create(ctx, t)
}

// RevokeByHash revokes in Postgres first, then synchronously evicts the cache
// entry. If the Postgres write fails the cache is NOT evicted.
func (c *RefreshTokenCache) RevokeByHash(ctx context.Context, hash string) error {
	if err := c.source.RevokeByHash(ctx, hash); err != nil {
		return err
	}
	c.mu.Lock()
	delete(c.data, hash)
	c.mu.Unlock()
	return nil
}
