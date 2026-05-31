package client

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/coldair426/auth-server/internal/platform/httpx"
)

// ClientService는 client.Handler가 의존하는 서비스 인터페이스이다.
type ClientService interface {
	GetClient(ctx context.Context, clientID uuid.UUID) (domain.OAuthClient, error)
}

// Handler는 클라이언트 HTTP 핸들러를 담당한다.
type Handler struct {
	svc ClientService
}

// NewHandler는 의존성을 주입받아 Handler를 생성한다.
func NewHandler(svc ClientService) *Handler {
	return &Handler{svc: svc}
}

type clientResponse struct {
	ClientID     string  `json:"clientId"`
	Name         string  `json:"name"`
	LogoURL      *string `json:"logoUrl"`
	FaviconURL   *string `json:"faviconUrl"`
	GradientFrom string  `json:"gradientFrom"`
	GradientTo   string  `json:"gradientTo"`
	TextDark     bool    `json:"textDark"`
}

// GetClient: GET /clients/{clientId}
func (h *Handler) GetClient(w http.ResponseWriter, r *http.Request) {
	clientID, err := uuid.Parse(chi.URLParam(r, "clientId"))
	if err != nil {
		httpx.WriteError(w, http.StatusBadRequest, "유효하지 않은 clientId", "INVALID_REQUEST")
		return
	}

	c, err := h.svc.GetClient(r.Context(), clientID)
	if err != nil {
		httpx.DomainError(w, err)
		return
	}

	httpx.WriteJSON(w, http.StatusOK, clientResponse{
		ClientID:     c.ID.String(),
		Name:         c.Name,
		LogoURL:      c.LogoURL,
		FaviconURL:   c.FaviconURL,
		GradientFrom: c.GradientFrom,
		GradientTo:   c.GradientTo,
		TextDark:     c.TextDark,
	})
}
