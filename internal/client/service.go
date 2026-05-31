package client

import (
	"context"
	"fmt"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/google/uuid"
)

// Service는 OAuth 클라이언트 조회를 담당한다.
type Service struct {
	clients ClientRepository
}

// NewService는 의존성을 주입받아 Service를 생성한다.
func NewService(clients ClientRepository) *Service {
	return &Service{clients: clients}
}

// GetClient는 clientID로 OAuthClient를 조회한다.
func (s *Service) GetClient(ctx context.Context, clientID uuid.UUID) (domain.OAuthClient, error) {
	c, err := s.clients.FindByID(ctx, clientID)
	if err != nil {
		return domain.OAuthClient{}, fmt.Errorf("client.Service.GetClient: %w", err)
	}
	return c, nil
}
