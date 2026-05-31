package auth_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"testing"
	"time"

	"github.com/coldair426/auth-server/internal/auth"
	"github.com/coldair426/auth-server/internal/domain"
	"github.com/coldair426/auth-server/internal/platform/oauth"
	"github.com/google/uuid"
)

// ─── 모의 구현체 ────────────────────────────────────────────────────────────

type mockUserRepo struct{}

func (m *mockUserRepo) Create(_ context.Context, u domain.User) (domain.User, error) {
	return u, nil
}
func (m *mockUserRepo) FindByID(_ context.Context, _ uuid.UUID) (domain.User, error) {
	return domain.User{}, domain.ErrUserNotFound
}

type mockOAuthAccountRepo struct {
	existing *domain.OAuthAccount
}

func (m *mockOAuthAccountRepo) FindByProvider(_ context.Context, _ domain.Provider, _ string) (domain.OAuthAccount, error) {
	if m.existing == nil {
		return domain.OAuthAccount{}, domain.ErrOAuthAccountNotFound
	}
	return *m.existing, nil
}
func (m *mockOAuthAccountRepo) Upsert(_ context.Context, a domain.OAuthAccount) (domain.OAuthAccount, error) {
	return a, nil
}

type mockClientRepo struct {
	client domain.OAuthClient
}

func (m *mockClientRepo) FindByID(_ context.Context, _ uuid.UUID) (domain.OAuthClient, error) {
	return m.client, nil
}

type mockRefreshTokenRepo struct {
	tokens  map[string]domain.RefreshToken
	revoked []string
	created []domain.RefreshToken
}

func newMockRefreshTokenRepo(tokens ...domain.RefreshToken) *mockRefreshTokenRepo {
	m := &mockRefreshTokenRepo{tokens: make(map[string]domain.RefreshToken)}
	for _, t := range tokens {
		m.tokens[t.TokenHash] = t
	}
	return m
}

func (m *mockRefreshTokenRepo) Create(_ context.Context, t domain.RefreshToken) (domain.RefreshToken, error) {
	m.tokens[t.TokenHash] = t
	m.created = append(m.created, t)
	return t, nil
}

func (m *mockRefreshTokenRepo) FindByHash(_ context.Context, hash string) (domain.RefreshToken, error) {
	t, ok := m.tokens[hash]
	if !ok {
		return domain.RefreshToken{}, domain.ErrRefreshTokenNotFound
	}
	return t, nil
}

func (m *mockRefreshTokenRepo) RevokeByHash(_ context.Context, hash string) error {
	t, ok := m.tokens[hash]
	if !ok {
		return domain.ErrRefreshTokenNotFound
	}
	now := time.Now()
	t.RevokedAt = &now
	m.tokens[hash] = t
	m.revoked = append(m.revoked, hash)
	return nil
}

type mockMembershipRepo struct {
	existing *domain.Membership
	created  []domain.Membership
}

func (m *mockMembershipRepo) FindByUserAndClient(_ context.Context, _, _ uuid.UUID) (domain.Membership, error) {
	if m.existing == nil {
		return domain.Membership{}, domain.ErrMembershipNotFound
	}
	return *m.existing, nil
}
func (m *mockMembershipRepo) Create(_ context.Context, mem domain.Membership) error {
	m.created = append(m.created, mem)
	return nil
}

type mockOAuthProvider struct {
	providerUserID string
	email          string
}

func (m *mockOAuthProvider) AuthCodeURL(state, redirectURI string) string {
	return "https://example.com/auth?state=" + state + "&redirect_uri=" + redirectURI
}
func (m *mockOAuthProvider) Exchange(_ context.Context, _, _ string) (string, string, error) {
	return m.providerUserID, m.email, nil
}

// testHashToken은 테스트에서 raw 토큰의 SHA-256 해시를 계산한다.
func testHashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// ─── 테스트: HandleCallback ─────────────────────────────────────────────────

func TestHandleCallback(t *testing.T) {
	jwtMgr := newTestJWTManager(t)

	existingUserID := uuid.New()
	clientID := uuid.New()

	existingAcct := &domain.OAuthAccount{
		ID:         uuid.New(),
		UserID:     existingUserID,
		Provider:   domain.ProviderGoogle,
		ProviderID: "google-provider-123",
	}

	existingMembership := &domain.Membership{
		UserID:   existingUserID,
		ClientID: clientID,
		JoinedAt: time.Now(),
	}

	cases := []struct {
		name          string
		acct          *domain.OAuthAccount
		membership    *domain.Membership
		wantIsNewUser bool
		wantNeedsJoin bool
	}{
		{
			name:          "신규 사용자 - 미가입",
			acct:          nil,
			membership:    nil,
			wantIsNewUser: true,
			wantNeedsJoin: true,
		},
		{
			name:          "기존 사용자 - 미가입",
			acct:          existingAcct,
			membership:    nil,
			wantIsNewUser: false,
			wantNeedsJoin: true,
		},
		{
			name:          "기존 사용자 - 가입완료",
			acct:          existingAcct,
			membership:    existingMembership,
			wantIsNewUser: false,
			wantNeedsJoin: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const stateVal = "test-state-xyz"
			states := auth.NewStateStore()
			states.Store(stateVal, auth.StateData{
				ClientID:    clientID,
				RedirectURI: "http://example.com/callback",
			}, time.Minute)

			svc := auth.NewService(
				&mockUserRepo{},
				&mockOAuthAccountRepo{existing: tc.acct},
				&mockClientRepo{},
				newMockRefreshTokenRepo(),
				&mockMembershipRepo{existing: tc.membership},
				states,
				jwtMgr,
				oauth.Registry{
					domain.ProviderGoogle: &mockOAuthProvider{
						providerUserID: "google-provider-123",
						email:          "test@example.com",
					},
				},
			)

			result, err := svc.HandleCallback(context.Background(), domain.ProviderGoogle, "auth-code", stateVal)
			if err != nil {
				t.Fatalf("HandleCallback 실패: %v", err)
			}

			if result.IsNewUser != tc.wantIsNewUser {
				t.Errorf("IsNewUser: got %v, want %v", result.IsNewUser, tc.wantIsNewUser)
			}
			if result.NeedsJoin != tc.wantNeedsJoin {
				t.Errorf("NeedsJoin: got %v, want %v", result.NeedsJoin, tc.wantNeedsJoin)
			}
			if result.AccessToken == "" {
				t.Error("AccessToken이 비어있음")
			}
			if result.RawRefreshToken == "" {
				t.Error("RawRefreshToken이 비어있음")
			}
		})
	}
}

func TestHandleCallback_InvalidState(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	svc := auth.NewService(
		&mockUserRepo{}, &mockOAuthAccountRepo{}, &mockClientRepo{},
		newMockRefreshTokenRepo(), &mockMembershipRepo{},
		auth.NewStateStore(), jwtMgr, oauth.Registry{},
	)

	_, err := svc.HandleCallback(context.Background(), domain.ProviderGoogle, "code", "non-existent-state")
	if err == nil {
		t.Error("잘못된 state에서 오류가 반환되지 않음")
	}
}

// ─── 테스트: Refresh ────────────────────────────────────────────────────────

func TestRefresh(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	userID := uuid.New()
	clientID := uuid.New()

	const rawToken = "test-raw-refresh-token"
	hash := testHashToken(rawToken)
	now := time.Now()

	base := domain.RefreshToken{
		ID:        uuid.New(),
		UserID:    userID,
		ClientID:  clientID,
		TokenHash: hash,
		ExpiresAt: now.Add(30 * 24 * time.Hour),
	}

	expiredToken := base
	expiredToken.ExpiresAt = now.Add(-time.Minute)

	revokedAt := now.Add(-time.Second)
	revokedToken := base
	revokedToken.RevokedAt = &revokedAt

	cases := []struct {
		name    string
		token   *domain.RefreshToken
		wantErr error
	}{
		{"유효한 토큰 - 교체 성공", &base, nil},
		{"만료된 토큰", &expiredToken, domain.ErrRefreshTokenExpired},
		{"폐기된 토큰", &revokedToken, domain.ErrRefreshTokenRevoked},
		{"존재하지 않는 토큰", nil, domain.ErrRefreshTokenNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var tokenRepo *mockRefreshTokenRepo
			if tc.token != nil {
				tokenRepo = newMockRefreshTokenRepo(*tc.token)
			} else {
				tokenRepo = newMockRefreshTokenRepo()
			}

			svc := auth.NewService(
				&mockUserRepo{}, &mockOAuthAccountRepo{}, &mockClientRepo{},
				tokenRepo, &mockMembershipRepo{},
				auth.NewStateStore(), jwtMgr, oauth.Registry{},
			)

			result, err := svc.Refresh(context.Background(), rawToken)

			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Errorf("오류 불일치: got %v, want %v", err, tc.wantErr)
				}
				return
			}

			if err != nil {
				t.Fatalf("Refresh 실패: %v", err)
			}
			if result.AccessToken == "" {
				t.Error("AccessToken이 비어있음")
			}
			if result.RawRefreshToken == "" {
				t.Error("새 RawRefreshToken이 비어있음")
			}
			if result.RawRefreshToken == rawToken {
				t.Error("새 토큰이 기존 토큰과 동일함 (교체되지 않음)")
			}
			if len(tokenRepo.revoked) == 0 {
				t.Error("기존 토큰이 폐기되지 않음")
			}
			if len(tokenRepo.created) == 0 {
				t.Error("새 토큰이 저장되지 않음")
			}
		})
	}
}

// ─── 테스트: BuildLoginURL redirectURI 검증 ─────────────────────────────────

func TestBuildLoginURL_RedirectURIAllowlist(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	clientID := uuid.New()

	clientRepo := &mockClientRepo{
		client: domain.OAuthClient{
			ID:                  clientID,
			AllowedRedirectURIs: []string{"http://allowed.example.com/callback"},
		},
	}
	oauthReg := oauth.Registry{
		domain.ProviderGoogle: &mockOAuthProvider{},
	}

	svc := auth.NewService(
		&mockUserRepo{}, &mockOAuthAccountRepo{}, clientRepo,
		newMockRefreshTokenRepo(), &mockMembershipRepo{},
		auth.NewStateStore(), jwtMgr, oauthReg,
	)

	cases := []struct {
		name        string
		redirectURI string
		wantErr     bool
	}{
		{"허용된 redirectURI", "http://allowed.example.com/callback", false},
		{"허용되지 않은 redirectURI", "http://evil.example.com/callback", true},
		{"빈 redirectURI", "", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			url, err := svc.BuildLoginURL(context.Background(), domain.ProviderGoogle, clientID, tc.redirectURI)

			if tc.wantErr {
				if err == nil {
					t.Error("오류가 반환되어야 하는데 성공함")
				}
				if !errors.Is(err, domain.ErrInvalidRedirectURI) {
					t.Errorf("ErrInvalidRedirectURI 예상, got: %v", err)
				}
				return
			}

			if err != nil {
				t.Fatalf("BuildLoginURL 실패: %v", err)
			}
			if url == "" {
				t.Error("빈 URL 반환됨")
			}
		})
	}
}

// ─── 테스트: Join 멱등성 ────────────────────────────────────────────────────

func TestJoin_Idempotent(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	userID := uuid.New()
	clientID := uuid.New()

	membership := &domain.Membership{UserID: userID, ClientID: clientID, JoinedAt: time.Now()}
	membershipRepo := &mockMembershipRepo{existing: membership}

	svc := auth.NewService(
		&mockUserRepo{}, &mockOAuthAccountRepo{}, &mockClientRepo{},
		newMockRefreshTokenRepo(), membershipRepo,
		auth.NewStateStore(), jwtMgr, oauth.Registry{},
	)

	// 이미 가입된 경우 오류 없이 성공해야 한다.
	if err := svc.Join(context.Background(), userID, clientID); err != nil {
		t.Errorf("이미 가입된 사용자에게 Join이 오류를 반환함: %v", err)
	}
	if len(membershipRepo.created) != 0 {
		t.Error("이미 가입된 경우 중복 멤버십이 생성됨")
	}
}

// ─── 테스트: Logout 멱등성 ──────────────────────────────────────────────────

func TestLogout_Idempotent(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	svc := auth.NewService(
		&mockUserRepo{}, &mockOAuthAccountRepo{}, &mockClientRepo{},
		newMockRefreshTokenRepo(), // 토큰 없음
		&mockMembershipRepo{},
		auth.NewStateStore(), jwtMgr, oauth.Registry{},
	)

	// 존재하지 않는 토큰에 대한 Logout은 오류 없이 성공해야 한다.
	if err := svc.Logout(context.Background(), "non-existent-token"); err != nil {
		t.Errorf("존재하지 않는 토큰 Logout이 오류를 반환함: %v", err)
	}
}
