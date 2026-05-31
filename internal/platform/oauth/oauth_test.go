package oauth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"golang.org/x/oauth2"
)

// mockServer는 /token과 /userinfo 경로를 처리하는 테스트 서버를 생성한다.
func mockServer(t *testing.T, tokenResp, userinfoResp any) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/token":
			if err := json.NewEncoder(w).Encode(tokenResp); err != nil {
				t.Errorf("token 응답 인코딩 실패: %v", err)
			}
		case "/userinfo":
			if err := json.NewEncoder(w).Encode(userinfoResp); err != nil {
				t.Errorf("userinfo 응답 인코딩 실패: %v", err)
			}
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	return srv
}

// testEndpoint는 인증 스타일 자동 감지를 비활성화한 테스트용 엔드포인트를 반환한다.
func testEndpoint(baseURL string) oauth2.Endpoint {
	return oauth2.Endpoint{
		AuthURL:   baseURL + "/auth",
		TokenURL:  baseURL + "/token",
		AuthStyle: oauth2.AuthStyleInHeader,
	}
}

func TestGoogleProvider_Exchange(t *testing.T) {
	const (
		wantProviderID = "google-user-123"
		wantEmail      = "user@gmail.com"
	)

	srv := mockServer(t,
		map[string]any{
			"access_token": "fake-google-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		},
		map[string]any{
			"sub":   wantProviderID,
			"email": wantEmail,
		},
	)

	p := &googleProvider{
		clientID:     "test-client-id",
		clientSecret: "test-secret",
		endpoint:     testEndpoint(srv.URL),
		userinfoURL:  srv.URL + "/userinfo",
	}

	providerID, email, err := p.Exchange(context.Background(), "test-code", "http://localhost/callback")
	if err != nil {
		t.Fatalf("Exchange 실패: %v", err)
	}
	if providerID != wantProviderID {
		t.Errorf("providerID 불일치: got %q, want %q", providerID, wantProviderID)
	}
	if email != wantEmail {
		t.Errorf("email 불일치: got %q, want %q", email, wantEmail)
	}
}

func TestGoogleProvider_AuthCodeURL(t *testing.T) {
	p := &googleProvider{
		clientID:    "test-client-id",
		endpoint:    testEndpoint("https://accounts.google.com"),
		userinfoURL: "https://www.googleapis.com/oauth2/v3/userinfo",
	}

	url := p.AuthCodeURL("state-xyz", "http://localhost/callback")
	if url == "" {
		t.Error("AuthCodeURL가 빈 문자열을 반환함")
	}
}

func TestKakaoProvider_Exchange(t *testing.T) {
	const (
		wantProviderID = "987654321"
		wantEmail      = "kakao@kakao.com"
	)

	srv := mockServer(t,
		map[string]any{
			"access_token": "fake-kakao-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		},
		map[string]any{
			"id": 987654321,
			"kakao_account": map[string]any{
				"email": wantEmail,
			},
		},
	)

	p := &kakaoProvider{
		clientID:     "test-client-id",
		clientSecret: "test-secret",
		endpoint:     testEndpoint(srv.URL),
		userinfoURL:  srv.URL + "/userinfo",
	}

	// kakao userinfo 호출은 httpClientFromContext를 사용하므로 주입한다.
	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, srv.Client())

	providerID, email, err := p.Exchange(ctx, "test-code", "http://localhost/callback")
	if err != nil {
		t.Fatalf("Exchange 실패: %v", err)
	}
	if providerID != wantProviderID {
		t.Errorf("providerID 불일치: got %q, want %q", providerID, wantProviderID)
	}
	if email != wantEmail {
		t.Errorf("email 불일치: got %q, want %q", email, wantEmail)
	}
}

func TestNaverProvider_Exchange(t *testing.T) {
	const (
		wantProviderID = "naver-user-abc"
		wantEmail      = "naver@naver.com"
	)

	srv := mockServer(t,
		map[string]any{
			"access_token": "fake-naver-token",
			"token_type":   "Bearer",
			"expires_in":   3600,
		},
		map[string]any{
			"response": map[string]any{
				"id":    wantProviderID,
				"email": wantEmail,
			},
		},
	)

	p := &naverProvider{
		clientID:     "test-client-id",
		clientSecret: "test-secret",
		endpoint:     testEndpoint(srv.URL),
		userinfoURL:  srv.URL + "/userinfo",
	}

	ctx := context.WithValue(context.Background(), oauth2.HTTPClient, srv.Client())

	providerID, email, err := p.Exchange(ctx, "test-code", "http://localhost/callback")
	if err != nil {
		t.Fatalf("Exchange 실패: %v", err)
	}
	if providerID != wantProviderID {
		t.Errorf("providerID 불일치: got %q, want %q", providerID, wantProviderID)
	}
	if email != wantEmail {
		t.Errorf("email 불일치: got %q, want %q", email, wantEmail)
	}
}
