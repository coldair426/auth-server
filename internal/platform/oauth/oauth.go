package oauth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/coldair426/auth-server/internal/platform/config"
)

// Provider는 OAuth2 제공자 인터페이스이다.
type Provider interface {
	AuthCodeURL(state, redirectURI string) string
	Exchange(ctx context.Context, code, redirectURI string) (providerUserID string, email string, err error)
}

// Registry는 도메인 Provider를 OAuth2 구현체에 매핑한다.
type Registry map[domain.Provider]Provider

// NewRegistry는 환경 변수 기반으로 Registry를 생성한다.
func NewRegistry(cfg *config.Config) Registry {
	return Registry{
		domain.ProviderGoogle: newGoogleProvider(cfg),
		domain.ProviderKakao:  newKakaoProvider(cfg),
		domain.ProviderNaver:  newNaverProvider(cfg),
	}
}

// httpClientFromContext는 컨텍스트에서 HTTP 클라이언트를 추출한다.
// oauth2.HTTPClient 키로 주입된 클라이언트가 없으면 기본 클라이언트를 반환한다.
func httpClientFromContext(ctx context.Context) *http.Client {
	if c, ok := ctx.Value(oauth2.HTTPClient).(*http.Client); ok {
		return c
	}
	return http.DefaultClient
}

// --- Google ---

type googleProvider struct {
	clientID     string
	clientSecret string
	endpoint     oauth2.Endpoint
	userinfoURL  string
}

func newGoogleProvider(cfg *config.Config) *googleProvider {
	return &googleProvider{
		clientID:     cfg.GoogleClientID,
		clientSecret: cfg.GoogleClientSecret,
		endpoint:     google.Endpoint,
		userinfoURL:  "https://www.googleapis.com/oauth2/v3/userinfo",
	}
}

func (p *googleProvider) oauthConfig(redirectURI string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		Endpoint:     p.endpoint,
		Scopes:       []string{"openid", "email"},
		RedirectURL:  redirectURI,
	}
}

func (p *googleProvider) AuthCodeURL(state, redirectURI string) string {
	return p.oauthConfig(redirectURI).AuthCodeURL(state, oauth2.AccessTypeOnline)
}

func (p *googleProvider) Exchange(ctx context.Context, code, redirectURI string) (string, string, error) {
	cfg := p.oauthConfig(redirectURI)
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return "", "", fmt.Errorf("google: 코드 교환 실패: %w", err)
	}

	// cfg.Client는 컨텍스트의 HTTP 클라이언트를 기반으로 Bearer 토큰을 자동 추가한다.
	client := cfg.Client(ctx, token)
	resp, err := client.Get(p.userinfoURL)
	if err != nil {
		return "", "", fmt.Errorf("google: 사용자 정보 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	var info struct {
		Sub   string `json:"sub"`
		Email string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", "", fmt.Errorf("google: 사용자 정보 파싱 실패: %w", err)
	}
	return info.Sub, info.Email, nil
}

// --- Kakao ---

type kakaoProvider struct {
	clientID     string
	clientSecret string
	endpoint     oauth2.Endpoint
	userinfoURL  string
}

func newKakaoProvider(cfg *config.Config) *kakaoProvider {
	return &kakaoProvider{
		clientID:     cfg.KakaoClientID,
		clientSecret: cfg.KakaoClientSecret,
		endpoint: oauth2.Endpoint{
			AuthURL:  "https://kauth.kakao.com/oauth/authorize",
			TokenURL: "https://kauth.kakao.com/oauth/token",
		},
		userinfoURL: "https://kapi.kakao.com/v2/user/me",
	}
}

func (p *kakaoProvider) oauthConfig(redirectURI string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		Endpoint:     p.endpoint,
		RedirectURL:  redirectURI,
	}
}

func (p *kakaoProvider) AuthCodeURL(state, redirectURI string) string {
	return p.oauthConfig(redirectURI).AuthCodeURL(state)
}

func (p *kakaoProvider) Exchange(ctx context.Context, code, redirectURI string) (string, string, error) {
	cfg := p.oauthConfig(redirectURI)
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return "", "", fmt.Errorf("kakao: 코드 교환 실패: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userinfoURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("kakao: 요청 생성 실패: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := httpClientFromContext(ctx).Do(req)
	if err != nil {
		return "", "", fmt.Errorf("kakao: 사용자 정보 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	var info struct {
		ID           int64 `json:"id"`
		KakaoAccount struct {
			Email string `json:"email"`
		} `json:"kakao_account"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", "", fmt.Errorf("kakao: 사용자 정보 파싱 실패: %w", err)
	}
	return fmt.Sprintf("%d", info.ID), info.KakaoAccount.Email, nil
}

// --- Naver ---

type naverProvider struct {
	clientID     string
	clientSecret string
	endpoint     oauth2.Endpoint
	userinfoURL  string
}

func newNaverProvider(cfg *config.Config) *naverProvider {
	return &naverProvider{
		clientID:     cfg.NaverClientID,
		clientSecret: cfg.NaverClientSecret,
		endpoint: oauth2.Endpoint{
			AuthURL:  "https://nid.naver.com/oauth2.0/authorize",
			TokenURL: "https://nid.naver.com/oauth2.0/token",
		},
		userinfoURL: "https://openapi.naver.com/v1/nid/me",
	}
}

func (p *naverProvider) oauthConfig(redirectURI string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.clientID,
		ClientSecret: p.clientSecret,
		Endpoint:     p.endpoint,
		RedirectURL:  redirectURI,
	}
}

func (p *naverProvider) AuthCodeURL(state, redirectURI string) string {
	return p.oauthConfig(redirectURI).AuthCodeURL(state)
}

func (p *naverProvider) Exchange(ctx context.Context, code, redirectURI string) (string, string, error) {
	cfg := p.oauthConfig(redirectURI)
	token, err := cfg.Exchange(ctx, code)
	if err != nil {
		return "", "", fmt.Errorf("naver: 코드 교환 실패: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.userinfoURL, nil)
	if err != nil {
		return "", "", fmt.Errorf("naver: 요청 생성 실패: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	resp, err := httpClientFromContext(ctx).Do(req)
	if err != nil {
		return "", "", fmt.Errorf("naver: 사용자 정보 요청 실패: %w", err)
	}
	defer resp.Body.Close()

	var info struct {
		Response struct {
			ID    string `json:"id"`
			Email string `json:"email"`
		} `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return "", "", fmt.Errorf("naver: 사용자 정보 파싱 실패: %w", err)
	}
	return info.Response.ID, info.Response.Email, nil
}
