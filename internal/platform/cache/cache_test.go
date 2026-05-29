package cache

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/google/uuid"
)

// ---- test helpers ----

func stubClient(id uuid.UUID) domain.OAuthClient {
	return domain.OAuthClient{
		ID:                  id,
		Name:                "Test App",
		GradientFrom:        "#000",
		GradientTo:          "#fff",
		AllowedRedirectURIs: []string{"http://localhost:3000/cb"},
	}
}

func activeToken(hash string) domain.RefreshToken {
	return domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    uuid.New(),
		ClientID:  uuid.New(),
		TokenHash: hash,
		ExpiresAt: time.Now().Add(30 * 24 * time.Hour),
	}
}

// ---- mock client source ----

type mockClientSource struct {
	calls int64
	fn    func(ctx context.Context, id uuid.UUID) (domain.OAuthClient, error)
}

func (m *mockClientSource) FindByID(ctx context.Context, id uuid.UUID) (domain.OAuthClient, error) {
	atomic.AddInt64(&m.calls, 1)
	return m.fn(ctx, id)
}

// ---- mock token source ----

type mockTokenSource struct {
	findCalls   int64
	revokeCalls int64
	findFn      func(ctx context.Context, hash string) (domain.RefreshToken, error)
	revokeFn    func(ctx context.Context, hash string) error
	createFn    func(ctx context.Context, t domain.RefreshToken) (domain.RefreshToken, error)
}

func (m *mockTokenSource) FindByHash(ctx context.Context, hash string) (domain.RefreshToken, error) {
	atomic.AddInt64(&m.findCalls, 1)
	return m.findFn(ctx, hash)
}

func (m *mockTokenSource) RevokeByHash(ctx context.Context, hash string) error {
	atomic.AddInt64(&m.revokeCalls, 1)
	return m.revokeFn(ctx, hash)
}

func (m *mockTokenSource) Create(ctx context.Context, t domain.RefreshToken) (domain.RefreshToken, error) {
	if m.createFn != nil {
		return m.createFn(ctx, t)
	}
	return t, nil
}

// ===== ClientCache =====

func TestClientCache_ColdStart_FallsThrough(t *testing.T) {
	id := uuid.New()
	src := &mockClientSource{fn: func(_ context.Context, _ uuid.UUID) (domain.OAuthClient, error) {
		return stubClient(id), nil
	}}
	c := NewClientCache(src)

	got, err := c.Get(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id {
		t.Errorf("ID: got %v, want %v", got.ID, id)
	}
	if atomic.LoadInt64(&src.calls) != 1 {
		t.Errorf("source calls: got %d, want 1", src.calls)
	}
}

func TestClientCache_Hit_SkipsSource(t *testing.T) {
	id := uuid.New()
	src := &mockClientSource{fn: func(_ context.Context, _ uuid.UUID) (domain.OAuthClient, error) {
		return stubClient(id), nil
	}}
	c := NewClientCache(src)

	if _, err := c.Get(context.Background(), id); err != nil {
		t.Fatal("prime:", err)
	}
	if _, err := c.Get(context.Background(), id); err != nil {
		t.Fatal("second:", err)
	}

	if atomic.LoadInt64(&src.calls) != 1 {
		t.Errorf("source calls after cache warm: got %d, want 1", src.calls)
	}
}

func TestClientCache_Invalidate_CausesNextMiss(t *testing.T) {
	id := uuid.New()
	src := &mockClientSource{fn: func(_ context.Context, _ uuid.UUID) (domain.OAuthClient, error) {
		return stubClient(id), nil
	}}
	c := NewClientCache(src)

	if _, err := c.Get(context.Background(), id); err != nil {
		t.Fatal("prime:", err)
	}
	c.Invalidate(id)
	if _, err := c.Get(context.Background(), id); err != nil {
		t.Fatal("post-invalidate:", err)
	}

	if atomic.LoadInt64(&src.calls) != 2 {
		t.Errorf("source calls after invalidate: got %d, want 2", src.calls)
	}
}

func TestClientCache_Invalidate_AbsentKey_NoOp(t *testing.T) {
	c := NewClientCache(&mockClientSource{fn: func(_ context.Context, _ uuid.UUID) (domain.OAuthClient, error) {
		return domain.OAuthClient{}, nil
	}})
	// must not panic or error
	c.Invalidate(uuid.New())
}

func TestClientCache_SourceError_Propagated(t *testing.T) {
	sentinel := errors.New("db unavailable")
	src := &mockClientSource{fn: func(_ context.Context, _ uuid.UUID) (domain.OAuthClient, error) {
		return domain.OAuthClient{}, sentinel
	}}
	c := NewClientCache(src)

	_, err := c.Get(context.Background(), uuid.New())
	if !errors.Is(err, sentinel) {
		t.Errorf("error: got %v, want %v", err, sentinel)
	}
}

func TestClientCache_SourceError_NotCached(t *testing.T) {
	id := uuid.New()
	calls := int64(0)
	sentinel := errors.New("transient error")
	src := &mockClientSource{fn: func(_ context.Context, _ uuid.UUID) (domain.OAuthClient, error) {
		n := atomic.AddInt64(&calls, 1)
		if n == 1 {
			return domain.OAuthClient{}, sentinel
		}
		return stubClient(id), nil
	}}
	c := NewClientCache(src)

	if _, err := c.Get(context.Background(), id); !errors.Is(err, sentinel) {
		t.Fatalf("first call: expected sentinel, got %v", err)
	}
	// second call should retry the source, not serve a cached error
	if _, err := c.Get(context.Background(), id); err != nil {
		t.Fatalf("second call: expected success, got %v", err)
	}
	if atomic.LoadInt64(&calls) != 2 {
		t.Errorf("source calls: got %d, want 2", calls)
	}
}

func TestClientCache_FindByID_DelegatesToGet(t *testing.T) {
	id := uuid.New()
	src := &mockClientSource{fn: func(_ context.Context, _ uuid.UUID) (domain.OAuthClient, error) {
		return stubClient(id), nil
	}}
	c := NewClientCache(src)

	got, err := c.FindByID(context.Background(), id)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != id {
		t.Errorf("ID mismatch")
	}
}

func TestClientCache_ConcurrentAccess(t *testing.T) {
	id := uuid.New()
	src := &mockClientSource{fn: func(_ context.Context, _ uuid.UUID) (domain.OAuthClient, error) {
		return stubClient(id), nil
	}}
	c := NewClientCache(src)

	const goroutines = 200
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			switch n % 3 {
			case 0:
				c.Invalidate(id)
			case 1:
				if _, err := c.Get(context.Background(), id); err != nil {
					t.Errorf("Get: %v", err)
				}
			case 2:
				if _, err := c.FindByID(context.Background(), id); err != nil {
					t.Errorf("FindByID: %v", err)
				}
			}
		}(i)
	}
	wg.Wait()
}

// ===== RefreshTokenCache =====

func TestRefreshTokenCache_ColdStart_FallsThrough(t *testing.T) {
	tok := activeToken("hash-cold")
	src := &mockTokenSource{
		findFn: func(_ context.Context, _ string) (domain.RefreshToken, error) {
			return tok, nil
		},
	}
	c := NewRefreshTokenCache(src)

	got, err := c.FindByHash(context.Background(), "hash-cold")
	if err != nil {
		t.Fatal(err)
	}
	if got.TokenHash != tok.TokenHash {
		t.Errorf("hash: got %q, want %q", got.TokenHash, tok.TokenHash)
	}
	if atomic.LoadInt64(&src.findCalls) != 1 {
		t.Errorf("source calls: got %d, want 1", src.findCalls)
	}
}

func TestRefreshTokenCache_Hit_SkipsSource(t *testing.T) {
	tok := activeToken("hash-hit")
	src := &mockTokenSource{
		findFn: func(_ context.Context, _ string) (domain.RefreshToken, error) {
			return tok, nil
		},
	}
	c := NewRefreshTokenCache(src)

	if _, err := c.FindByHash(context.Background(), "hash-hit"); err != nil {
		t.Fatal("prime:", err)
	}
	if _, err := c.FindByHash(context.Background(), "hash-hit"); err != nil {
		t.Fatal("second:", err)
	}
	if atomic.LoadInt64(&src.findCalls) != 1 {
		t.Errorf("source calls after cache warm: got %d, want 1", src.findCalls)
	}
}

func TestRefreshTokenCache_RevokedToken_NotCached(t *testing.T) {
	revokedAt := time.Now().Add(-time.Second)
	tok := domain.RefreshToken{
		ID:        uuid.New(),
		TokenHash: "hash-revoked",
		ExpiresAt: time.Now().Add(time.Hour),
		RevokedAt: &revokedAt,
	}
	src := &mockTokenSource{
		findFn: func(_ context.Context, _ string) (domain.RefreshToken, error) {
			return tok, nil
		},
	}
	c := NewRefreshTokenCache(src)

	for i := 0; i < 3; i++ {
		if _, err := c.FindByHash(context.Background(), "hash-revoked"); err != nil {
			t.Fatal(err)
		}
	}
	if atomic.LoadInt64(&src.findCalls) != 3 {
		t.Errorf("revoked token was cached: source calls %d, want 3", src.findCalls)
	}
}

func TestRefreshTokenCache_ExpiredToken_NotCached(t *testing.T) {
	tok := domain.RefreshToken{
		ID:        uuid.New(),
		TokenHash: "hash-expired",
		ExpiresAt: time.Now().Add(-time.Second),
	}
	src := &mockTokenSource{
		findFn: func(_ context.Context, _ string) (domain.RefreshToken, error) {
			return tok, nil
		},
	}
	c := NewRefreshTokenCache(src)

	for i := 0; i < 3; i++ {
		if _, err := c.FindByHash(context.Background(), "hash-expired"); err != nil {
			t.Fatal(err)
		}
	}
	if atomic.LoadInt64(&src.findCalls) != 3 {
		t.Errorf("expired token was cached: source calls %d, want 3", src.findCalls)
	}
}

func TestRefreshTokenCache_Revoke_EvictsCache(t *testing.T) {
	tok := activeToken("hash-evict")
	src := &mockTokenSource{
		findFn: func(_ context.Context, _ string) (domain.RefreshToken, error) {
			return tok, nil
		},
		revokeFn: func(_ context.Context, _ string) error { return nil },
	}
	c := NewRefreshTokenCache(src)

	// warm the cache
	if _, err := c.FindByHash(context.Background(), "hash-evict"); err != nil {
		t.Fatal("prime:", err)
	}
	if err := c.RevokeByHash(context.Background(), "hash-evict"); err != nil {
		t.Fatal("revoke:", err)
	}
	// next find must fall through to source
	if _, err := c.FindByHash(context.Background(), "hash-evict"); err != nil {
		t.Fatal("post-revoke:", err)
	}
	if atomic.LoadInt64(&src.findCalls) != 2 {
		t.Errorf("cache not evicted after revoke: source calls %d, want 2", src.findCalls)
	}
}

func TestRefreshTokenCache_RevokeFail_CachePreserved(t *testing.T) {
	tok := activeToken("hash-fail")
	dbErr := errors.New("postgres down")
	src := &mockTokenSource{
		findFn: func(_ context.Context, _ string) (domain.RefreshToken, error) {
			return tok, nil
		},
		revokeFn: func(_ context.Context, _ string) error { return dbErr },
	}
	c := NewRefreshTokenCache(src)

	// warm the cache
	if _, err := c.FindByHash(context.Background(), "hash-fail"); err != nil {
		t.Fatal("prime:", err)
	}
	// revoke fails
	if err := c.RevokeByHash(context.Background(), "hash-fail"); !errors.Is(err, dbErr) {
		t.Fatalf("expected dbErr, got %v", err)
	}
	// cache entry must still be present: second find should NOT call source
	if _, err := c.FindByHash(context.Background(), "hash-fail"); err != nil {
		t.Fatal("post-failed-revoke:", err)
	}
	if atomic.LoadInt64(&src.findCalls) != 1 {
		t.Errorf("cache was evicted despite Postgres failure: source calls %d, want 1", src.findCalls)
	}
}

func TestRefreshTokenCache_Revoke_PostgresCalledBeforeEvict(t *testing.T) {
	tok := activeToken("hash-order")
	var order []string
	var mu sync.Mutex
	record := func(s string) {
		mu.Lock()
		order = append(order, s)
		mu.Unlock()
	}
	src := &mockTokenSource{
		findFn: func(_ context.Context, _ string) (domain.RefreshToken, error) {
			record("find")
			return tok, nil
		},
		revokeFn: func(_ context.Context, _ string) error {
			record("postgres-revoke")
			return nil
		},
	}
	c := NewRefreshTokenCache(src)

	if _, err := c.FindByHash(context.Background(), "hash-order"); err != nil {
		t.Fatal(err)
	}
	if err := c.RevokeByHash(context.Background(), "hash-order"); err != nil {
		t.Fatal(err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(order) < 2 || order[0] != "find" || order[1] != "postgres-revoke" {
		t.Errorf("unexpected call order: %v", order)
	}
}

func TestRefreshTokenCache_Create_WriteThrough(t *testing.T) {
	tok := activeToken("hash-create")
	src := &mockTokenSource{
		createFn: func(_ context.Context, t domain.RefreshToken) (domain.RefreshToken, error) {
			return t, nil
		},
		findFn: func(_ context.Context, _ string) (domain.RefreshToken, error) {
			return tok, nil
		},
	}
	c := NewRefreshTokenCache(src)

	created, err := c.Create(context.Background(), tok)
	if err != nil {
		t.Fatal(err)
	}
	if created.TokenHash != tok.TokenHash {
		t.Errorf("hash mismatch after create")
	}
	// Create does not pre-populate cache; first FindByHash must hit source
	if _, err := c.FindByHash(context.Background(), "hash-create"); err != nil {
		t.Fatal(err)
	}
	if atomic.LoadInt64(&src.findCalls) != 1 {
		t.Errorf("source calls: got %d, want 1", src.findCalls)
	}
}

func TestRefreshTokenCache_SourceError_Propagated(t *testing.T) {
	sentinel := errors.New("not found")
	src := &mockTokenSource{
		findFn: func(_ context.Context, _ string) (domain.RefreshToken, error) {
			return domain.RefreshToken{}, sentinel
		},
	}
	c := NewRefreshTokenCache(src)

	_, err := c.FindByHash(context.Background(), "missing")
	if !errors.Is(err, sentinel) {
		t.Errorf("error: got %v, want %v", err, sentinel)
	}
}

func TestRefreshTokenCache_ConcurrentAccess(t *testing.T) {
	tok := activeToken("hash-concurrent")
	src := &mockTokenSource{
		findFn: func(_ context.Context, _ string) (domain.RefreshToken, error) {
			return tok, nil
		},
		revokeFn: func(_ context.Context, _ string) error { return nil },
	}
	c := NewRefreshTokenCache(src)

	const goroutines = 200
	var wg sync.WaitGroup
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(n int) {
			defer wg.Done()
			switch n % 3 {
			case 0:
				if _, err := c.FindByHash(context.Background(), "hash-concurrent"); err != nil {
					t.Errorf("FindByHash: %v", err)
				}
			case 1:
				c.RevokeByHash(context.Background(), "hash-concurrent") //nolint:errcheck
			case 2:
				c.Create(context.Background(), tok) //nolint:errcheck
			}
		}(i)
	}
	wg.Wait()
}
