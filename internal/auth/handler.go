package auth

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/coldair426/auth-server/internal/platform/httpx"
)

// AuthService는 auth.Handler가 의존하는 서비스 인터페이스이다.
type AuthService interface {
	BuildLoginURL(ctx context.Context, provider domain.Provider, clientID uuid.UUID, redirectURI string) (string, error)
	HandleCallback(ctx context.Context, provider domain.Provider, code, state string) (CallbackResult, error)
	Join(ctx context.Context, userID, clientID uuid.UUID) error
	Refresh(ctx context.Context, rawRefreshToken string) (RefreshResult, error)
	Logout(ctx context.Context, rawRefreshToken string) error
}

// Handler는 인증 HTTP 핸들러를 담당한다.
type Handler struct {
	svc          AuthService
	cookieDomain string
}

// NewHandler는 의존성을 주입받아 Handler를 생성한다.
func NewHandler(svc AuthService, cookieDomain string) *Handler {
	return &Handler{svc: svc, cookieDomain: cookieDomain}
}

// GetLoginURL: GET /auth/{provider}/url?clientId=...&redirectUri=...
func (h *Handler) GetLoginURL(w http.ResponseWriter, r *http.Request) {
	provider, err := domain.ParseProvider(chi.URLParam(r, "provider"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "유효하지 않은 provider", "INVALID_REQUEST")
		return
	}

	clientID, err := uuid.Parse(r.URL.Query().Get("clientId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "유효하지 않은 clientId", "INVALID_REQUEST")
		return
	}

	redirectURI := r.URL.Query().Get("redirectUri")
	if redirectURI == "" {
		httpx.WriteError(w, http.StatusBadRequest, "redirectUri가 필요합니다", "INVALID_REQUEST")
		return
	}

	url, err := h.svc.BuildLoginURL(r.Context(), provider, clientID, redirectURI)
	if err != nil {
		httpx.DomainError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, map[string]string{"url": url})
}

// HandleCallback: POST /auth/{provider}/callback
type callbackRequest struct {
	Code  string `json:"code"`
	State string `json:"state"`
}

type callbackResponse struct {
	AccessToken string `json:"accessToken"`
	NeedsJoin   bool   `json:"needsJoin"`
	IsNewUser   bool   `json:"isNewUser"`
}

func (h *Handler) HandleCallback(w http.ResponseWriter, r *http.Request) {
	provider, err := domain.ParseProvider(chi.URLParam(r, "provider"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "유효하지 않은 provider", "INVALID_REQUEST")
		return
	}

	var req callbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "요청 본문이 유효하지 않습니다", "INVALID_REQUEST")
		return
	}
	if req.Code == "" || req.State == "" {
		httpx.WriteError(w, http.StatusBadRequest, "code와 state가 필요합니다", "INVALID_REQUEST")
		return
	}

	result, err := h.svc.HandleCallback(r.Context(), provider, req.Code, req.State)
	if err != nil {
		httpx.DomainError(w, err)
		return
	}

	httpx.SetAccessTokenCookie(w, result.AccessToken, h.cookieDomain)
	httpx.SetRefreshCookie(w, result.RawRefreshToken, h.cookieDomain)

	httpx.WriteJSON(w, http.StatusOK, callbackResponse{
		AccessToken: result.AccessToken,
		NeedsJoin:   result.NeedsJoin,
		IsNewUser:   result.IsNewUser,
	})
}

// Join: POST /auth/join (인증 필요)
type joinRequest struct {
	ClientID string `json:"clientId"`
}

func (h *Handler) Join(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpx.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "인증 정보가 없습니다", "UNAUTHORIZED")
		return
	}

	var req joinRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "요청 본문이 유효하지 않습니다", "INVALID_REQUEST")
		return
	}

	clientID, err := uuid.Parse(req.ClientID)
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "유효하지 않은 clientId", "INVALID_REQUEST")
		return
	}

	if err := h.svc.Join(r.Context(), userID, clientID); err != nil {
		httpx.DomainError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}

// Refresh: POST /auth/refresh
type refreshResponse struct {
	AccessToken string `json:"accessToken"`
}

func (h *Handler) Refresh(w http.ResponseWriter, r *http.Request) {
	c, err := r.Cookie("refresh")
	if err != nil {
		httpx.WriteError(w, http.StatusUnauthorized, "refresh token이 없습니다", "UNAUTHORIZED")
		return
	}

	result, err := h.svc.Refresh(r.Context(), c.Value)
	if err != nil {
		httpx.DomainError(w, err)
		return
	}

	httpx.SetAccessTokenCookie(w, result.AccessToken, h.cookieDomain)
	httpx.SetRefreshCookie(w, result.RawRefreshToken, h.cookieDomain)

	httpx.WriteJSON(w, http.StatusOK, refreshResponse{AccessToken: result.AccessToken})
}

// Logout: POST /auth/logout
func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	if c, err := r.Cookie("refresh"); err == nil {
		_ = h.svc.Logout(r.Context(), c.Value)
	}
	httpx.ClearAuthCookies(w, h.cookieDomain)
	w.WriteHeader(http.StatusOK)
}
