package domain

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type PolicyType string

const (
	PolicyTerms      PolicyType = "TERMS"
	PolicyPrivacy    PolicyType = "PRIVACY"
	PolicyThirdParty PolicyType = "THIRD_PARTY"
)

func ParsePolicyType(s string) (PolicyType, error) {
	switch p := PolicyType(s); p {
	case PolicyTerms, PolicyPrivacy, PolicyThirdParty:
		return p, nil
	default:
		return "", fmt.Errorf("%w: %q", ErrInvalidPolicyType, s)
	}
}

type Consent struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	PolicyType  PolicyType
	Version     string
	ServiceID   *string
	ConsentedAt time.Time
}
