//go:build integration

package integration_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/coldair426/auth-server/internal/auth"
	"github.com/coldair426/auth-server/internal/client"
	"github.com/coldair426/auth-server/internal/consent"
	"github.com/coldair426/auth-server/internal/domain"
	"github.com/coldair426/auth-server/internal/platform/cache"
	"github.com/coldair426/auth-server/internal/platform/config"
	"github.com/coldair426/auth-server/internal/platform/httpx"
	"github.com/coldair426/auth-server/internal/platform/jwt"
	"github.com/coldair426/auth-server/internal/platform/oauth"
	"github.com/coldair426/auth-server/internal/platform/postgres"
)

// ─── 상수 ────────────────────────────────────────────────────────────────────

const (
	seedClientID = "00000000-0000-0000-0000-000000000001"
	redirectURI  = "http://localhost:3000/auth/callback"
)

// ─── 모의 OAuth 제공자 ────────────────────────────────────────────────────────

// testOAuthProvider는 실제 HTTP 엔드포인트를 호출하지 않는 테스트용 OAuth 제공자이다.
type testOAuthProvider struct {
	providerUserID string
	email          string
}

func (p *testOAuthProvider) AuthCodeURL(state, _ string) string {
	u := &url.URL{
		Scheme:   "https",
		Host:     "accounts.google.com",
		Path:     "/o/oauth2/auth",
		RawQuery: "state=" + url.QueryEscape(state),
	}
	return u.String()
}

func (p *testOAuthProvider) Exchange(_ context.Context, _, _ string) (string, string, error) {
	return p.providerUserID, p.email, nil
}

// ─── 테스트 헬퍼 ─────────────────────────────────────────────────────────────

func newTestJWTManager(t *testing.T) *jwt.Manager {
	t.Helper()
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("RSA 키 생성 실패: %v", err)
	}
	dir := t.TempDir()

	privFile, _ := os.CreateTemp(dir, "private*.pem")
	privBytes, _ := x509.MarshalPKCS8PrivateKey(privateKey)
	_ = pem.Encode(privFile, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes})
	privFile.Close()

	pubFile, _ := os.CreateTemp(dir, "public*.pem")
	pubBytes, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	_ = pem.Encode(pubFile, &pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes})
	pubFile.Close()

	m, err := jwt.New(&config.Config{
		JWTPrivateKeyPath: privFile.Name(),
		JWTPublicKeyPath:  pubFile.Name(),
	})
	if err != nil {
		t.Fatalf("JWT Manager 생성 실패: %v", err)
	}
	return m
}

// runMigrations는 db/migrations/*.up.sql 파일을 멱등적으로 실행한다.
// 이 테스트의 작업 디렉토리는 internal/integration/이므로 ../../db/migrations 경로를 사용한다.
func runMigrations(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	migrationsDir := "../../db/migrations"

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("마이그레이션 디렉토리 읽기 실패: %v", err)
	}

	var upFiles []string
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".up.sql") {
			upFiles = append(upFiles, filepath.Join(migrationsDir, e.Name()))
		}
	}
	sort.Strings(upFiles)

	for _, f := range upFiles {
		sql, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("마이그레이션 파일 읽기 실패 %s: %v", f, err)
		}
		if _, execErr := pool.Exec(ctx, makeIdempotent(string(sql))); execErr != nil {
			t.Fatalf("마이그레이션 실행 실패 %s: %v", filepath.Base(f), execErr)
		}
	}
}

// makeIdempotent는 SQL 문을 IF NOT EXISTS 구문을 추가해 멱등적으로 변환한다.
func makeIdempotent(sql string) string {
	lines := strings.Split(sql, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "CREATE TABLE ") && !strings.Contains(trimmed, "IF NOT EXISTS"):
			lines[i] = strings.Replace(line, "CREATE TABLE ", "CREATE TABLE IF NOT EXISTS ", 1)
		case strings.HasPrefix(trimmed, "CREATE UNIQUE INDEX ") && !strings.Contains(trimmed, "IF NOT EXISTS"):
			lines[i] = strings.Replace(line, "CREATE UNIQUE INDEX ", "CREATE UNIQUE INDEX IF NOT EXISTS ", 1)
		case strings.HasPrefix(trimmed, "CREATE INDEX ") && !strings.Contains(trimmed, "IF NOT EXISTS"):
			lines[i] = strings.Replace(line, "CREATE INDEX ", "CREATE INDEX IF NOT EXISTS ", 1)
		}
	}
	return strings.Join(lines, "\n")
}

func insertSeedClient(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	_, err := pool.Exec(ctx, `
		INSERT INTO oauth_clients (
			client_id, name, gradient_from, gradient_to, text_dark, allowed_redirect_uris
		) VALUES (
			$1, 'Test App', '#6366f1', '#8b5cf6', false, ARRAY[$2]::text[]
		) ON CONFLICT (client_id) DO NOTHING
	`, seedClientID, redirectURI)
	if err != nil {
		t.Fatalf("시드 클라이언트 삽입 실패: %v", err)
	}
}

func truncateUsers(t *testing.T, ctx context.Context, pool *pgxpool.Pool) {
	t.Helper()
	if _, err := pool.Exec(ctx, "TRUNCATE users CASCADE"); err != nil {
		t.Logf("테이블 정리 실패 (무시됨): %v", err)
	}
}

func buildRouter(
	logger *slog.Logger,
	jwtMgr *jwt.Manager,
	authH *auth.Handler,
	clientH *client.Handler,
	consentH *consent.Handler,
) http.Handler {
	r := chi.NewRouter()
	r.Use(httpx.Recovery(logger))

	r.Get("/clients/{clientId}", clientH.GetClient)

	r.Route("/auth", func(r chi.Router) {
		r.Get("/{provider}/url", authH.GetLoginURL)
		r.Post("/{provider}/callback", authH.HandleCallback)
		r.Post("/refresh", authH.Refresh)
		r.Post("/logout", authH.Logout)
		r.Group(func(r chi.Router) {
			r.Use(httpx.AuthMiddleware(jwtMgr))
			r.Post("/join", authH.Join)
		})
	})

	r.Get("/users/{userId}/consents", consentH.ListConsents)
	r.Group(func(r chi.Router) {
		r.Use(httpx.AuthMiddleware(jwtMgr))
		r.Post("/consents", consentH.RecordConsents)
	})

	return r
}

// ─── 통합 테스트 ──────────────────────────────────────────────────────────────

// TestFullAuthSequence는 실제 Postgres를 대상으로 전체 인증 시퀀스를 검증한다.
// 실행 전 TEST_DATABASE_URL 환경 변수가 설정되어 있어야 한다.
//
// 실행 방법:
//
//	TEST_DATABASE_URL="postgres://user:pass@localhost:5432/auth_test?sslmode=disable" \
//	go test -tags=integration ./internal/integration/ -v
func TestFullAuthSequence(t *testing.T) {
	dbURL := os.Getenv("TEST_DATABASE_URL")
	if dbURL == "" {
		t.Skip("TEST_DATABASE_URL이 설정되지 않아 통합 테스트를 건너뜁니다")
	}

	ctx := context.Background()

	// ─── DB 연결 ──────────────────────────────────────────────────────────────
	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		t.Fatalf("DB 연결 실패: %v", err)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		t.Fatalf("DB ping 실패: %v", err)
	}

	// ─── 마이그레이션 및 시드 ─────────────────────────────────────────────────
	runMigrations(t, ctx, pool)
	insertSeedClient(t, ctx, pool)
	t.Cleanup(func() { truncateUsers(t, ctx, pool) })

	// ─── 서비스 스택 구성 ────────────────────────────────────────────────────
	jwtMgr := newTestJWTManager(t)

	oauthReg := oauth.Registry{
		domain.ProviderGoogle: &testOAuthProvider{
			providerUserID: "google-integration-test-123",
			email:          "integration@example.com",
		},
	}

	q := postgres.New(pool)
	userRepo := postgres.NewUserRepo(q)
	oauthAcctRepo := postgres.NewOAuthAccountRepo(q)
	oauthClientRepo := postgres.NewOAuthClientRepo(q)
	refreshTokenRepo := postgres.NewRefreshTokenRepo(q)
	membershipRepo := postgres.NewMembershipRepo(q)
	consentRepo := postgres.NewConsentRepo(q)

	clientCache := cache.NewClientCache(oauthClientRepo)
	tokenCache := cache.NewRefreshTokenCache(refreshTokenRepo)
	stateStore := auth.NewStateStore()

	authSvc := auth.NewService(
		userRepo, oauthAcctRepo, clientCache,
		tokenCache, membershipRepo,
		stateStore, jwtMgr, oauthReg,
	)
	clientSvc := client.NewService(clientCache)
	consentSvc := consent.NewService(consentRepo)

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelError}))

	authH := auth.NewHandler(authSvc, "")
	clientH := client.NewHandler(clientSvc)
	consentH := consent.NewHandler(consentSvc)

	srv := httptest.NewServer(buildRouter(logger, jwtMgr, authH, clientH, consentH))
	defer srv.Close()

	httpClient := &http.Client{
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// ─── Step 1: Login URL 획득 ───────────────────────────────────────────────
	t.Log("Step 1: Login URL 획득")

	loginURL := srv.URL + "/auth/google/url" +
		"?clientId=" + seedClientID +
		"&redirectUri=" + url.QueryEscape(redirectURI)

	resp, err := httpClient.Get(loginURL)
	if err != nil {
		t.Fatalf("Login URL 요청 실패: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("Login URL 상태 코드 불일치: got %d", resp.StatusCode)
	}

	var urlResp struct {
		URL string `json:"url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&urlResp); err != nil {
		t.Fatalf("Login URL 응답 파싱 실패: %v", err)
	}

	parsedLoginURL, err := url.Parse(urlResp.URL)
	if err != nil {
		t.Fatalf("state URL 파싱 실패: %v", err)
	}
	stateVal := parsedLoginURL.Query().Get("state")
	if stateVal == "" {
		t.Fatal("state 값이 비어있음")
	}
	t.Logf("  state 획득: %s…", stateVal[:8])

	// ─── Step 2: Callback ─────────────────────────────────────────────────────
	t.Log("Step 2: OAuth Callback 처리")

	cbBody, _ := json.Marshal(map[string]string{"code": "test-auth-code", "state": stateVal})
	cbReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/auth/google/callback", bytes.NewReader(cbBody))
	cbReq.Header.Set("Content-Type", "application/json")

	cbResp, err := httpClient.Do(cbReq)
	if err != nil {
		t.Fatalf("Callback 요청 실패: %v", err)
	}
	defer cbResp.Body.Close()
	if cbResp.StatusCode != http.StatusOK {
		t.Fatalf("Callback 상태 코드 불일치: got %d", cbResp.StatusCode)
	}

	var callbackResult struct {
		AccessToken string `json:"accessToken"`
		NeedsJoin   bool   `json:"needsJoin"`
		IsNewUser   bool   `json:"isNewUser"`
	}
	if err := json.NewDecoder(cbResp.Body).Decode(&callbackResult); err != nil {
		t.Fatalf("Callback 응답 파싱 실패: %v", err)
	}
	if callbackResult.AccessToken == "" {
		t.Fatal("accessToken이 비어있음")
	}
	if !callbackResult.IsNewUser {
		t.Error("신규 사용자여야 하는데 isNewUser=false")
	}
	if !callbackResult.NeedsJoin {
		t.Error("미가입 상태여야 하는데 needsJoin=false")
	}
	t.Logf("  accessToken 발급 완료, isNewUser=%v needsJoin=%v",
		callbackResult.IsNewUser, callbackResult.NeedsJoin)

	accessToken := callbackResult.AccessToken

	var refreshToken string
	for _, c := range cbResp.Cookies() {
		if c.Name == "refresh" {
			refreshToken = c.Value
		}
	}
	if refreshToken == "" {
		t.Fatal("refresh 쿠키가 없음")
	}

	// ─── Step 3: Join ─────────────────────────────────────────────────────────
	t.Log("Step 3: 클라이언트 가입")

	joinBody, _ := json.Marshal(map[string]string{"clientId": seedClientID})
	joinReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/auth/join", bytes.NewReader(joinBody))
	joinReq.Header.Set("Authorization", "Bearer "+accessToken)
	joinReq.Header.Set("Content-Type", "application/json")

	joinResp, err := httpClient.Do(joinReq)
	if err != nil {
		t.Fatalf("Join 요청 실패: %v", err)
	}
	joinResp.Body.Close()
	if joinResp.StatusCode != http.StatusOK {
		t.Fatalf("Join 상태 코드 불일치: got %d", joinResp.StatusCode)
	}
	t.Log("  가입 완료")

	// ─── Step 4: Refresh (토큰 교체) ──────────────────────────────────────────
	t.Log("Step 4: 토큰 갱신")

	refreshReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/auth/refresh", nil)
	refreshReq.AddCookie(&http.Cookie{Name: "refresh", Value: refreshToken})

	refreshResp, err := httpClient.Do(refreshReq)
	if err != nil {
		t.Fatalf("Refresh 요청 실패: %v", err)
	}
	defer refreshResp.Body.Close()
	if refreshResp.StatusCode != http.StatusOK {
		t.Fatalf("Refresh 상태 코드 불일치: got %d", refreshResp.StatusCode)
	}

	var refreshResult struct {
		AccessToken string `json:"accessToken"`
	}
	if err := json.NewDecoder(refreshResp.Body).Decode(&refreshResult); err != nil {
		t.Fatalf("Refresh 응답 파싱 실패: %v", err)
	}
	if refreshResult.AccessToken == "" {
		t.Fatal("갱신된 accessToken이 비어있음")
	}

	var newRefreshToken string
	for _, c := range refreshResp.Cookies() {
		if c.Name == "refresh" {
			newRefreshToken = c.Value
		}
	}
	if newRefreshToken == "" {
		t.Fatal("새 refresh 쿠키가 없음")
	}
	if newRefreshToken == refreshToken {
		t.Error("refresh token이 교체되지 않음 (rotation 미동작)")
	}
	t.Logf("  새 accessToken 발급, refresh token 교체 완료")

	// 기존(폐기된) refresh token 재사용 불가 검증
	oldTokenReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/auth/refresh", nil)
	oldTokenReq.AddCookie(&http.Cookie{Name: "refresh", Value: refreshToken})
	oldTokenResp, err := httpClient.Do(oldTokenReq)
	if err != nil {
		t.Fatalf("기존 토큰 재사용 요청 실패: %v", err)
	}
	oldTokenResp.Body.Close()
	if oldTokenResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("폐기된 token 재사용 시 401 기대, got %d", oldTokenResp.StatusCode)
	}
	t.Log("  폐기된 토큰 재사용 차단 확인")

	// ─── Step 5: Logout ───────────────────────────────────────────────────────
	t.Log("Step 5: 로그아웃")

	logoutReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/auth/logout", nil)
	logoutReq.AddCookie(&http.Cookie{Name: "refresh", Value: newRefreshToken})

	logoutResp, err := httpClient.Do(logoutReq)
	if err != nil {
		t.Fatalf("Logout 요청 실패: %v", err)
	}
	logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("Logout 상태 코드 불일치: got %d", logoutResp.StatusCode)
	}

	// 로그아웃 후 토큰 재사용 불가 검증
	postLogoutReq, _ := http.NewRequest(http.MethodPost, srv.URL+"/auth/refresh", nil)
	postLogoutReq.AddCookie(&http.Cookie{Name: "refresh", Value: newRefreshToken})
	postLogoutResp, err := httpClient.Do(postLogoutReq)
	if err != nil {
		t.Fatalf("로그아웃 후 재사용 요청 실패: %v", err)
	}
	postLogoutResp.Body.Close()
	if postLogoutResp.StatusCode != http.StatusUnauthorized {
		t.Errorf("로그아웃 후 토큰 재사용 시 401 기대, got %d", postLogoutResp.StatusCode)
	}
	t.Log("  로그아웃 후 토큰 재사용 차단 확인")

	t.Log("✓ 전체 인증 시퀀스 성공: login → callback → join → refresh → logout")
}
