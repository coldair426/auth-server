package client_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/coldair426/auth-server/internal/client"
	"github.com/coldair426/auth-server/internal/domain"
)

// ─── 모의 서비스 ─────────────────────────────────────────────────────────────

type mockClientService struct {
	client domain.OAuthClient
	err    error
}

func (m *mockClientService) GetClient(_ context.Context, _ uuid.UUID) (domain.OAuthClient, error) {
	return m.client, m.err
}

// ─── 테스트 헬퍼 ─────────────────────────────────────────────────────────────

func newClientRouter(svc client.ClientService) http.Handler {
	h := client.NewHandler(svc)
	r := chi.NewRouter()
	r.Get("/clients/{clientId}", h.GetClient)
	return r
}

// ─── 테스트 ──────────────────────────────────────────────────────────────────

func TestGetClient_Happy(t *testing.T) {
	logo := "https://example.com/logo.png"
	clientID := uuid.New()

	svc := &mockClientService{
		client: domain.OAuthClient{
			ID:                  clientID,
			Name:                "테스트 앱",
			LogoURL:             &logo,
			FaviconURL:          nil,
			GradientFrom:        "#000000",
			GradientTo:          "#ffffff",
			TextDark:            true,
			AllowedRedirectURIs: []string{"http://example.com/callback"},
		},
	}

	req := httptest.NewRequest(http.MethodGet, "/clients/"+clientID.String(), nil)
	w := httptest.NewRecorder()
	newClientRouter(svc).ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("상태 코드 불일치: got %d, want %d", w.Code, http.StatusOK)
	}

	var resp map[string]any
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("JSON 파싱 실패: %v", err)
	}

	// 프론트엔드 계약 필드명 검증
	fields := []string{"clientId", "name", "logoUrl", "faviconUrl", "gradientFrom", "gradientTo", "textDark"}
	for _, f := range fields {
		if _, ok := resp[f]; !ok {
			t.Errorf("응답에 필드 %q 없음", f)
		}
	}
	if resp["clientId"] != clientID.String() {
		t.Errorf("clientId 불일치: got %v, want %v", resp["clientId"], clientID.String())
	}
	if resp["name"] != "테스트 앱" {
		t.Errorf("name 불일치: got %v", resp["name"])
	}
	// allowedRedirectUris는 노출하지 않아야 한다.
	if _, ok := resp["allowedRedirectUris"]; ok {
		t.Error("allowedRedirectUris가 응답에 노출됨")
	}
}

func TestGetClient_NotFound(t *testing.T) {
	svc := &mockClientService{err: domain.ErrClientNotFound}

	req := httptest.NewRequest(http.MethodGet, "/clients/"+uuid.New().String(), nil)
	w := httptest.NewRecorder()
	newClientRouter(svc).ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("상태 코드 불일치: got %d, want %d", w.Code, http.StatusNotFound)
	}

	var env map[string]any
	if err := json.NewDecoder(w.Body).Decode(&env); err != nil {
		t.Fatalf("오류 응답 파싱 실패: %v", err)
	}
	if env["code"] != "NOT_FOUND" {
		t.Errorf("오류 code 불일치: got %v", env["code"])
	}
}

func TestGetClient_InvalidUUID(t *testing.T) {
	svc := &mockClientService{}

	req := httptest.NewRequest(http.MethodGet, "/clients/not-a-uuid", nil)
	w := httptest.NewRecorder()
	newClientRouter(svc).ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("상태 코드 불일치: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}
