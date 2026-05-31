package consent

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/coldair426/auth-server/internal/platform/httpx"
)

// ConsentService는 consent.Handler가 의존하는 서비스 인터페이스이다.
type ConsentService interface {
	RecordConsents(ctx context.Context, userID uuid.UUID, items []ConsentItem, serviceID string) error
	ListConsents(ctx context.Context, userID uuid.UUID) ([]domain.Consent, error)
}

// Handler는 동의 HTTP 핸들러를 담당한다.
type Handler struct {
	svc ConsentService
}

// NewHandler는 의존성을 주입받아 Handler를 생성한다.
func NewHandler(svc ConsentService) *Handler {
	return &Handler{svc: svc}
}

type consentResponse struct {
	ID          string  `json:"id"`
	UserID      string  `json:"userId"`
	PolicyType  string  `json:"policyType"`
	Version     string  `json:"version"`
	ServiceID   *string `json:"serviceId"`
	ConsentedAt string  `json:"consentedAt"`
}

func toResponse(c domain.Consent) consentResponse {
	return consentResponse{
		ID:          c.ID.String(),
		UserID:      c.UserID.String(),
		PolicyType:  string(c.PolicyType),
		Version:     c.Version,
		ServiceID:   c.ServiceID,
		ConsentedAt: c.ConsentedAt.UTC().Format(time.RFC3339),
	}
}

// ListConsents: GET /users/{userId}/consents
func (h *Handler) ListConsents(w http.ResponseWriter, r *http.Request) {
	userID, err := uuid.Parse(chi.URLParam(r, "userId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "유효하지 않은 userId", "INVALID_REQUEST")
		return
	}

	consents, err := h.svc.ListConsents(r.Context(), userID)
	if err != nil {
		httpx.DomainError(w, err)
		return
	}

	resp := make([]consentResponse, len(consents))
	for i, c := range consents {
		resp[i] = toResponse(c)
	}
	httpx.WriteJSON(w, http.StatusOK, resp)
}

// RecordConsents: POST /consents (인증 필요)
type recordRequest struct {
	Items     []consentItemRequest `json:"items"`
	ServiceID string               `json:"serviceId"`
}

type consentItemRequest struct {
	PolicyType string `json:"policyType"`
	Version    string `json:"version"`
}

func (h *Handler) RecordConsents(w http.ResponseWriter, r *http.Request) {
	userID, ok := httpx.UserIDFromContext(r.Context())
	if !ok {
		httpx.WriteError(w, http.StatusUnauthorized, "인증 정보가 없습니다", "UNAUTHORIZED")
		return
	}

	var req recordRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "요청 본문이 유효하지 않습니다", "INVALID_REQUEST")
		return
	}

	items := make([]ConsentItem, 0, len(req.Items))
	for _, item := range req.Items {
		pt, err := domain.ParsePolicyType(item.PolicyType)
		if err != nil {
			httpx.DomainError(w, err)
			return
		}
		items = append(items, ConsentItem{PolicyType: pt, Version: item.Version})
	}

	if err := h.svc.RecordConsents(r.Context(), userID, items, req.ServiceID); err != nil {
		httpx.DomainError(w, err)
		return
	}

	w.WriteHeader(http.StatusOK)
}
