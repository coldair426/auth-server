package consent_test

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/coldair426/auth-server/internal/consent"
	"github.com/coldair426/auth-server/internal/domain"
	"github.com/coldair426/auth-server/internal/platform/config"
	"github.com/coldair426/auth-server/internal/platform/httpx"
	"github.com/coldair426/auth-server/internal/platform/jwt"
)

// ─── 모의 서비스 ─────────────────────────────────────────────────────────────

type mockConsentService struct {
	consents []domain.Consent
	err      error
}

func (m *mockConsentService) RecordConsents(_ context.Context, _ uuid.UUID, _ []consent.ConsentItem, _ string) error {
	return m.err
}
func (m *mockConsentService) ListConsents(_ context.Context, _ uuid.UUID) ([]domain.Consent, error) {
	return m.consents, m.err
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

func newConsentRouter(svc consent.ConsentService, jwtMgr *jwt.Manager) http.Handler {
	h := consent.NewHandler(svc)
	r := chi.NewRouter()
	r.Get("/users/{userId}/consents", h.ListConsents)
	r.Group(func(r chi.Router) {
		r.Use(httpx.AuthMiddleware(jwtMgr))
		r.Post("/consents", h.RecordConsents)
	})
	return r
}

func jsonBody(v any) *bytes.Buffer {
	b, _ := json.Marshal(v)
	return bytes.NewBuffer(b)
}

// ─── ListConsents ─────────────────────────────────────────────────────────────

func TestListConsents_Happy(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	sid := "service-001"
	userID := uuid.New()

	svc := &mockConsentService{
		consents: []domain.Consent{
			{
				ID:          uuid.New(),
				UserID:      userID,
				PolicyType:  domain.PolicyTerms,
				Version:     "1.0.0",
				ServiceID:   nil,
				ConsentedAt: time.Now().UTC(),
			},
			{
				ID:          uuid.New(),
				UserID:      userID,
				PolicyType:  domain.PolicyThirdParty,
				Version:     "1.0.0",
				ServiceID:   &sid,
				ConsentedAt: time.Now().UTC(),
			},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/users/"+userID.String()+"/consents", nil)
	w := httptest.NewRecorder()
	newConsentRouter(svc, jwtMgr).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("상태 코드 불일치: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp []map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("JSON 파싱 실패: %v", err)
	}
	if len(resp) != 2 {
		t.Errorf("응답 항목 수 불일치: got %d, want 2", len(resp))
	}

	// 프론트엔드 계약 필드명 검증
	fields := []string{"id", "userId", "policyType", "version", "serviceId", "consentedAt"}
	for _, f := range fields {
		if _, ok := resp[0][f]; !ok {
			t.Errorf("응답에 필드 %q 없음", f)
		}
	}
	if resp[0]["policyType"] != "TERMS" {
		t.Errorf("policyType 불일치: got %v", resp[0]["policyType"])
	}
}

func TestListConsents_InvalidUUID(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	svc := &mockConsentService{}

	req := httptest.NewRequest(http.MethodGet, "/users/not-a-uuid/consents", nil)
	w := httptest.NewRecorder()
	newConsentRouter(svc, jwtMgr).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("상태 코드 불일치: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ─── RecordConsents ───────────────────────────────────────────────────────────

func TestRecordConsents_Happy(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	svc := &mockConsentService{}

	userID := uuid.New()
	token, err := jwtMgr.IssueAccessToken(userID)
	if err != nil {
		t.Fatalf("토큰 발급 실패: %v", err)
	}

	body := jsonBody(map[string]any{
		"items": []map[string]string{
			{"policyType": "TERMS", "version": "1.0.0"},
			{"policyType": "PRIVACY", "version": "1.0.0"},
		},
		"serviceId": "",
	})
	req := httptest.NewRequest(http.MethodPost, "/consents", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newConsentRouter(svc, jwtMgr).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("상태 코드 불일치: got %d, want %d", w.Code, http.StatusOK)
	}
}

func TestRecordConsents_Unauthorized(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	svc := &mockConsentService{}

	body := jsonBody(map[string]any{
		"items": []map[string]string{
			{"policyType": "TERMS", "version": "1.0.0"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/consents", body)
	// Authorization 헤더 없음
	w := httptest.NewRecorder()
	newConsentRouter(svc, jwtMgr).ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("상태 코드 불일치: got %d, want %d", w.Code, http.StatusUnauthorized)
	}
}

func TestRecordConsents_InvalidPolicyType(t *testing.T) {
	jwtMgr := newTestJWTManager(t)
	svc := &mockConsentService{}

	userID := uuid.New()
	token, _ := jwtMgr.IssueAccessToken(userID)

	body := jsonBody(map[string]any{
		"items": []map[string]string{
			{"policyType": "INVALID_TYPE", "version": "1.0.0"},
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/consents", body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	newConsentRouter(svc, jwtMgr).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("상태 코드 불일치: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}
