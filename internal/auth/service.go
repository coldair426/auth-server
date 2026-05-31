package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/coldair426/auth-server/internal/platform/jwt"
	"github.com/coldair426/auth-server/internal/platform/oauth"
	"github.com/google/uuid"
)

const (
	stateTTL        = 10 * time.Minute
	refreshTokenTTL = 30 * 24 * time.Hour
	stateBytes      = 16
)

// CallbackResult는 HandleCallback의 반환값이다.
type CallbackResult struct {
	AccessToken     string
	RawRefreshToken string
	IsNewUser       bool
	NeedsJoin       bool
}

// RefreshResult는 Refresh의 반환값이다.
type RefreshResult struct {
	AccessToken     string
	RawRefreshToken string
}

// Service는 인증 관련 비즈니스 로직을 담당한다.
type Service struct {
	users       UserRepository
	oauthAccts  OAuthAccountRepository
	clients     OAuthClientRepository
	tokens      RefreshTokenRepository
	memberships MembershipRepository
	states      StateStore
	jwtMgr      *jwt.Manager
	oauthReg    oauth.Registry
}

// NewService는 의존성을 주입받아 Service를 생성한다.
func NewService(
	users UserRepository,
	oauthAccts OAuthAccountRepository,
	clients OAuthClientRepository,
	tokens RefreshTokenRepository,
	memberships MembershipRepository,
	states StateStore,
	jwtMgr *jwt.Manager,
	oauthReg oauth.Registry,
) *Service {
	return &Service{
		users:       users,
		oauthAccts:  oauthAccts,
		clients:     clients,
		tokens:      tokens,
		memberships: memberships,
		states:      states,
		jwtMgr:      jwtMgr,
		oauthReg:    oauthReg,
	}
}

// BuildLoginURL은 OAuth2 인증 URL을 반환한다.
// clientID와 redirectURI를 검증하고, 단기 state를 저장한다.
func (s *Service) BuildLoginURL(ctx context.Context, provider domain.Provider, clientID uuid.UUID, redirectURI string) (string, error) {
	client, err := s.clients.FindByID(ctx, clientID)
	if err != nil {
		return "", fmt.Errorf("auth.Service.BuildLoginURL: %w", err)
	}
	if err := client.ValidateRedirectURI(redirectURI); err != nil {
		return "", fmt.Errorf("auth.Service.BuildLoginURL: %w", err)
	}

	p, ok := s.oauthReg[provider]
	if !ok {
		return "", fmt.Errorf("auth.Service.BuildLoginURL: %w: %s", domain.ErrInvalidProvider, provider)
	}

	stateVal, err := generateState()
	if err != nil {
		return "", fmt.Errorf("auth.Service.BuildLoginURL: %w", err)
	}
	s.states.Store(stateVal, StateData{ClientID: clientID, RedirectURI: redirectURI}, stateTTL)

	return p.AuthCodeURL(stateVal, redirectURI), nil
}

// HandleCallback은 OAuth2 콜백을 처리하고 토큰을 발급한다.
// state 검증, 코드 교환, 사용자 조회/생성, 멤버십 확인, 토큰 발급을 순서대로 수행한다.
func (s *Service) HandleCallback(ctx context.Context, provider domain.Provider, code, state string) (CallbackResult, error) {
	stateData, ok := s.states.Consume(state)
	if !ok {
		return CallbackResult{}, fmt.Errorf("auth.Service.HandleCallback: 유효하지 않거나 만료된 state")
	}

	p, ok := s.oauthReg[provider]
	if !ok {
		return CallbackResult{}, fmt.Errorf("auth.Service.HandleCallback: %w: %s", domain.ErrInvalidProvider, provider)
	}

	providerUserID, email, err := p.Exchange(ctx, code, stateData.RedirectURI)
	if err != nil {
		return CallbackResult{}, fmt.Errorf("auth.Service.HandleCallback: 코드 교환 실패: %w", err)
	}

	var userID uuid.UUID
	var isNewUser bool

	acct, err := s.oauthAccts.FindByProvider(ctx, provider, providerUserID)
	switch {
	case err == nil:
		userID = acct.UserID
		isNewUser = false

	case errors.Is(err, domain.ErrOAuthAccountNotFound):
		newUserID, idErr := uuid.NewV7()
		if idErr != nil {
			return CallbackResult{}, fmt.Errorf("auth.Service.HandleCallback: UUID 생성 실패: %w", idErr)
		}
		newUser, createErr := s.users.Create(ctx, domain.User{ID: newUserID})
		if createErr != nil {
			return CallbackResult{}, fmt.Errorf("auth.Service.HandleCallback: 사용자 생성 실패: %w", createErr)
		}

		acctID, idErr := uuid.NewV7()
		if idErr != nil {
			return CallbackResult{}, fmt.Errorf("auth.Service.HandleCallback: UUID 생성 실패: %w", idErr)
		}
		var emailPtr *string
		if email != "" {
			emailPtr = &email
		}
		if _, upsertErr := s.oauthAccts.Upsert(ctx, domain.OAuthAccount{
			ID:         acctID,
			UserID:     newUser.ID,
			Provider:   provider,
			ProviderID: providerUserID,
			Email:      emailPtr,
		}); upsertErr != nil {
			return CallbackResult{}, fmt.Errorf("auth.Service.HandleCallback: OAuth 계정 생성 실패: %w", upsertErr)
		}

		userID = newUser.ID
		isNewUser = true

	default:
		return CallbackResult{}, fmt.Errorf("auth.Service.HandleCallback: %w", err)
	}

	_, err = s.memberships.FindByUserAndClient(ctx, userID, stateData.ClientID)
	needsJoin := errors.Is(err, domain.ErrMembershipNotFound)
	if err != nil && !needsJoin {
		return CallbackResult{}, fmt.Errorf("auth.Service.HandleCallback: 멤버십 조회 실패: %w", err)
	}

	accessToken, err := s.jwtMgr.IssueAccessToken(userID)
	if err != nil {
		return CallbackResult{}, fmt.Errorf("auth.Service.HandleCallback: access token 발급 실패: %w", err)
	}

	raw, hash, err := jwt.GenerateOpaqueRefreshToken()
	if err != nil {
		return CallbackResult{}, fmt.Errorf("auth.Service.HandleCallback: refresh token 생성 실패: %w", err)
	}

	tokenID, err := uuid.NewV7()
	if err != nil {
		return CallbackResult{}, fmt.Errorf("auth.Service.HandleCallback: UUID 생성 실패: %w", err)
	}
	if _, err = s.tokens.Create(ctx, domain.RefreshToken{
		ID:        tokenID,
		UserID:    userID,
		ClientID:  stateData.ClientID,
		TokenHash: hash,
		ExpiresAt: time.Now().Add(refreshTokenTTL),
	}); err != nil {
		return CallbackResult{}, fmt.Errorf("auth.Service.HandleCallback: refresh token 저장 실패: %w", err)
	}

	return CallbackResult{
		AccessToken:     accessToken,
		RawRefreshToken: raw,
		IsNewUser:       isNewUser,
		NeedsJoin:       needsJoin,
	}, nil
}

// Join은 사용자를 클라이언트에 가입시킨다. 이미 가입된 경우 멱등적으로 성공한다.
func (s *Service) Join(ctx context.Context, userID, clientID uuid.UUID) error {
	_, err := s.memberships.FindByUserAndClient(ctx, userID, clientID)
	if err == nil {
		return nil
	}
	if !errors.Is(err, domain.ErrMembershipNotFound) {
		return fmt.Errorf("auth.Service.Join: %w", err)
	}

	if err := s.memberships.Create(ctx, domain.Membership{
		UserID:   userID,
		ClientID: clientID,
		JoinedAt: time.Now(),
	}); err != nil {
		return fmt.Errorf("auth.Service.Join: %w", err)
	}
	return nil
}

// Refresh는 refresh token을 검증하고 새 토큰 쌍을 발급한다.
// 기존 토큰은 폐기(rotate)된다.
func (s *Service) Refresh(ctx context.Context, rawRefreshToken string) (RefreshResult, error) {
	hash := hashToken(rawRefreshToken)

	token, err := s.tokens.FindByHash(ctx, hash)
	if err != nil {
		return RefreshResult{}, fmt.Errorf("auth.Service.Refresh: %w", err)
	}
	if token.IsRevoked() {
		return RefreshResult{}, domain.ErrRefreshTokenRevoked
	}
	if token.IsExpired() {
		return RefreshResult{}, domain.ErrRefreshTokenExpired
	}

	if err := s.tokens.RevokeByHash(ctx, hash); err != nil {
		return RefreshResult{}, fmt.Errorf("auth.Service.Refresh: 기존 토큰 폐기 실패: %w", err)
	}

	raw, newHash, err := jwt.GenerateOpaqueRefreshToken()
	if err != nil {
		return RefreshResult{}, fmt.Errorf("auth.Service.Refresh: 새 refresh token 생성 실패: %w", err)
	}

	newID, err := uuid.NewV7()
	if err != nil {
		return RefreshResult{}, fmt.Errorf("auth.Service.Refresh: UUID 생성 실패: %w", err)
	}
	if _, err = s.tokens.Create(ctx, domain.RefreshToken{
		ID:        newID,
		UserID:    token.UserID,
		ClientID:  token.ClientID,
		TokenHash: newHash,
		ExpiresAt: time.Now().Add(refreshTokenTTL),
	}); err != nil {
		return RefreshResult{}, fmt.Errorf("auth.Service.Refresh: 새 refresh token 저장 실패: %w", err)
	}

	accessToken, err := s.jwtMgr.IssueAccessToken(token.UserID)
	if err != nil {
		return RefreshResult{}, fmt.Errorf("auth.Service.Refresh: access token 발급 실패: %w", err)
	}

	return RefreshResult{AccessToken: accessToken, RawRefreshToken: raw}, nil
}

// Logout은 refresh token을 폐기한다.
// 토큰이 이미 없거나 폐기된 경우에도 성공(멱등)한다.
func (s *Service) Logout(ctx context.Context, rawRefreshToken string) error {
	hash := hashToken(rawRefreshToken)
	if err := s.tokens.RevokeByHash(ctx, hash); err != nil {
		if errors.Is(err, domain.ErrRefreshTokenNotFound) {
			return nil
		}
		return fmt.Errorf("auth.Service.Logout: %w", err)
	}
	return nil
}

// hashToken은 raw refresh token의 SHA-256 해시를 hex 문자열로 반환한다.
func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}

// generateState는 crypto/rand 기반의 CSRF state 값을 생성한다.
func generateState() (string, error) {
	b := make([]byte, stateBytes)
	if _, err := rand.Read(b); err != nil {
		return "", fmt.Errorf("state 생성 실패: %w", err)
	}
	return hex.EncodeToString(b), nil
}
