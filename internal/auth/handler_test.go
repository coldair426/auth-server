package auth_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/coldair426/auth-server/internal/auth"
	"github.com/coldair426/auth-server/internal/domain"
	"github.com/coldair426/auth-server/internal/platform/httpx"
	"github.com/coldair426/auth-server/internal/platform/jwt"
)

// ─── 모의 서비스 ─────────────────────────────────────────────────────────────

type mockAuthService struct {
	loginURL       string
	loginURLErr    error
	callbackResult auth.CallbackResult
	callbackErr    error
	joinErr        error
	refreshResult  auth.RefreshResult
	refreshErr     error
	logoutErr      error
}

func (m *mockAuthService) BuildLoginURL(_ context.Context, _ domain.Provider, _ uuid.UUID, _ string) (string, error) {
	return m.loginURL, m.loginURLErr
}
func (m *mockAuthService) HandleCallback(_ context.Context, _ domain.Provider, _, _ string) (auth.CallbackResult, error) {
	return m.callbackResult, m.callbackErr
}
func (m *mockAuthService) Join(_ context.Context, _, _ uuid.UUID) error {
	return m.joinErr
}
func (m *mockAuthService) Refresh(_ context.Context, _ string) (auth.RefreshResult, error) {
	return m.refreshResult, m.refreshErr
}
func (m *mockAuthService) Logout(_ context.Context, _ string) error {
	return m.logoutErr
}

// ─── 테스트 헬퍼 ─────────────────────────────────────────────────────────────

func newAuthRouter(svc auth.AuthService, jwtMgr *jwt.Manager) http.Handler {
	h := auth.NewHandler(svc, "")
	r := chi.NewRouter()
	r.Get("/auth/{provider}/url", h.GetLoginURL)
	r.Post("/auth/{provider}/callback", h.HandleCallback)
	r.Post("/auth/refresh", h.Refresh)
	r.Post("/auth/logout", h.Logout)
	r.Group(func(r chi.Router) {
		r.Use(httpx.AuthMiddleware(jwtMgr))
		r.Post("/auth/join", h.Join)
	})
	return r
}

func jsonBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

// ─── GetLoginURL ─────────────────────────────────────────────────────────────

func TestGetLoginURL_Happy(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	svc := &mockAuthService{loginURL: "https://accounts.google.com/o/oauth2/auth?state=xyz"}

	req := httptest.NewRequest(http.MethodGet,
		"/auth/google/url?clientId="+uuid.New().String()+"&redirectUri=http://example.com/cb",
		nil,
	)
	w := httptest.NewRecorder()
	newAuthRouter(svc, jwtMgr).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("상태 코드 불일치: got %d, want %d", w.Code, http.StatusOK)
	}
	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("JSON 파싱 실패: %v", err)
	}
	if resp["url"] == "" {
		t.Error("url 필드가 비어있음")
	}
}

func TestGetLoginURL_InvalidRedirectURI(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	svc := &mockAuthService{loginURLErr: domain.ErrInvalidRedirectURI}

	req := httptest.NewRequest(http.MethodGet,
		"/auth/google/url?clientId="+uuid.New().String()+"&redirectUri=http://evil.com/cb",
		nil,
	)
	w := httptest.NewRecorder()
	newAuthRouter(svc, jwtMgr).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("상태 코드 불일치: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ─── HandleCallback ───────────────────────────────────────────────────────────

func TestHandleCallback_Happy(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	svc := &mockAuthService{
		callbackResult: auth.CallbackResult{
			AccessToken:     "test-access-token",
			RawRefreshToken: "test-refresh-token",
			IsNewUser:       true,
			NeedsJoin:       true,
		},
	}

	body := jsonBody(map[string]string{"code": "auth-code", "state": "state-xyz"})
	req := httptest.NewRequest(http.MethodPost, "/auth/google/callback", body)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newAuthRouter(svc, jwtMgr).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("상태 코드 불일치: got %d, want %d", w.Code, http.StatusOK)
	}

	// 응답 JSON 필드명 확인
	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("JSON 파싱 실패: %v", err)
	}
	for _, field := range []string{"accessToken", "needsJoin", "isNewUser"} {
		if _, ok := resp[field]; !ok {
			t.Errorf("응답에 필드 %q 없음", field)
		}
	}
	if resp["accessToken"] != "test-access-token" {
		t.Errorf("accessToken 불일치: got %v", resp["accessToken"])
	}

	// 쿠키 설정 확인
	var hasAccess, hasRefresh bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "access_token" {
			hasAccess = true
		}
		if c.Name == "refresh" {
			hasRefresh = true
		}
	}
	if !hasAccess {
		t.Error("access_token 쿠키가 설정되지 않음")
	}
	if !hasRefresh {
		t.Error("refresh 쿠키가 설정되지 않음")
	}
}

func TestHandleCallback_MissingBody(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	svc := &mockAuthService{}

	req := httptest.NewRequest(http.MethodPost, "/auth/google/callback",
		strings.NewReader(`{"code":"","state":""}`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newAuthRouter(svc, jwtMgr).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("상태 코드 불일치: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ─── Join ─────────────────────────────────────────────────────────────────────

func TestJoin_Happy(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	svc := &mockAuthService{}

	userID := uuid.New()
	token, err := jwtMgr.IssueAccessToken(userID)
	if err != nil {
		t.Fatalf("토큰 발급 실패: %v", err)
	}

	body := jsonBody(map[string]string{"clientId": uuid.New().String()})
	req := httptest.NewRequest(http.MethodPost, "/auth/join", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newAuthRouter(svc, jwtMgr).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("상태 코드 불일치: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestJoin_Unauthorized(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	svc := &mockAuthService{}

	body := jsonBody(map[string]string{"clientId": uuid.New().String()})
	req := httptest.NewRequest(http.MethodPost, "/auth/join", body)
	// Authorization 헤더 없음
	w := httptest.NewRecorder()
	newAuthRouter(svc, jwtMgr).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("상태 코드 불일치: got %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// ─── Refresh ──────────────────────────────────────────────────────────────────

func TestRefresh_Happy(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	svc := &mockAuthService{
		refreshResult: auth.RefreshResult{
			AccessToken:     "new-access-token",
			RawRefreshToken: "new-refresh-token",
		},
	}

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh", Value: "old-refresh-token"})
	w := httptest.NewRecorder()
	newAuthRouter(svc, jwtMgr).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("상태 코드 불일치: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]string
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("JSON 파싱 실패: %v", err)
	}
	if resp["accessToken"] != "new-access-token" {
		t.Errorf("accessToken 불일치: got %v", resp["accessToken"])
	}
}

func TestRefresh_NoCookie(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	svc := &mockAuthService{}

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	// refresh 쿠키 없음
	w := httptest.NewRecorder()
	newAuthRouter(svc, jwtMgr).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("상태 코드 불일치: got %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRefresh_ExpiredToken(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	svc := &mockAuthService{refreshErr: domain.ErrRefreshTokenExpired}

	req := httptest.NewRequest(http.MethodPost, "/auth/refresh", nil)
	req.AddCookie(&http.Cookie{Name: "refresh", Value: "expired-token"})
	w := httptest.NewRecorder()
	newAuthRouter(svc, jwtMgr).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("상태 코드 불일치: got %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

// ─── Logout ───────────────────────────────────────────────────────────────────

func TestLogout_Happy(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	svc := &mockAuthService{}

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	req.AddCookie(&http.Cookie{Name: "refresh", Value: "refresh-token"})
	w := httptest.NewRecorder()
	newAuthRouter(svc, jwtMgr).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("상태 코드 불일치: got %d, want %d", w.Code, http.StatusOK)
	}

	// 쿠키가 삭제(MaxAge=-1)되었는지 확인
	var clearedAccess, clearedRefresh bool
	for _, c := range w.Result().Cookies() {
		if c.Name == "access_token" && c.MaxAge < 0 {
			clearedAccess = true
		}
		if c.Name == "refresh" && c.MaxAge < 0 {
			clearedRefresh = true
		}
	}
	if !clearedAccess {
		t.Error("access_token 쿠키가 삭제되지 않음")
	}
	if !clearedRefresh {
		t.Error("refresh 쿠키가 삭제되지 않음")
	}
}

func TestLogout_NoCookie(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	svc := &mockAuthService{}

	req := httptest.NewRequest(http.MethodPost, "/auth/logout", nil)
	// 쿠키 없어도 200 OK (멱등)
	w := httptest.NewRecorder()
	newAuthRouter(svc, jwtMgr).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("쿠키 없는 로그아웃 상태 코드 불일치: got %d, want %d", w.Code, http.StatusOK)
	}
}
