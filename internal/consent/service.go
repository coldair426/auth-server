package consent

import (
	"context"
	"fmt"
	"time"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/google/uuid"
)

// ConsentItem은 동의 항목 하나를 나타낸다.
type ConsentItem struct {
	PolicyType domain.PolicyType
	Version    string
}

// Service는 사용자 동의 관련 비즈니스 로직을 담당한다.
type Service struct {
	consents ConsentRepository
}

// NewService는 의존성을 주입받아 Service를 생성한다.
func NewService(consents ConsentRepository) *Service {
	return &Service{consents: consents}
}

// RecordConsents는 사용자의 동의 항목을 일괄 저장한다.
// THIRD_PARTY 유형의 동의에는 serviceID가 필요하다.
func (s *Service) RecordConsents(ctx context.Context, userID uuid.UUID, items []ConsentItem, serviceID string) error {
	for _, item := range items {
		id, err := uuid.NewV7()
		if err != nil {
			return fmt.Errorf("consent.Service.RecordConsents: UUID 생성 실패: %w", err)
		}

		var sid *string
		if item.PolicyType == domain.PolicyThirdParty && serviceID != "" {
			sid = &serviceID
		}

		c := domain.Consent{
			ID:          id,
			UserID:      userID,
			PolicyType:  item.PolicyType,
			Version:     item.Version,
			ServiceID:   sid,
			ConsentedAt: time.Now(),
		}
		if err := s.consents.Insert(ctx, c); err != nil {
			return fmt.Errorf("consent.Service.RecordConsents: %w", err)
		}
	}
	return nil
}

// ListConsents는 사용자의 전체 동의 목록을 반환한다.
func (s *Service) ListConsents(ctx context.Context, userID uuid.UUID) ([]domain.Consent, error) {
	cs, err := s.consents.ListByUserID(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("consent.Service.ListConsents: %w", err)
	}
	return cs, nil
}
