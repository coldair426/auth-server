package domain

import (
	"errors"
	"time"

	"github.com/google/uuid"
)

var ErrConsentNotFound = errors.New("consent not found")

type PolicyType string

const (
	PolicyTerms      PolicyType = "TERMS"
	PolicyPrivacy    PolicyType = "PRIVACY"
	PolicyThirdParty PolicyType = "THIRD_PARTY"
)

type Consent struct {
	ID          uuid.UUID
	UserID      uuid.UUID
	PolicyType  PolicyType
	Version     string
	ServiceID   *string
	ConsentedAt time.Time
}
