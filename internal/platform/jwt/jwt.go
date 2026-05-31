package jwt

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"fmt"
	"os"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/coldair426/auth-server/internal/platform/config"
)

const (
	issuer         = "auth-server"
	accessTokenTTL = 15 * time.Minute
)

// Manager는 JWT 발급 및 검증을 담당한다.
type Manager struct {
	privateKey *rsa.PrivateKey
	publicKey  *rsa.PublicKey
}

// New는 설정에 명시된 경로에서 RSA 키를 로드하여 Manager를 생성한다.
func New(cfg *config.Config) (*Manager, error) {
	privBytes, err := os.ReadFile(cfg.JWTPrivateKeyPath)
	if err != nil {
		return nil, fmt.Errorf("jwt: private key 파일 읽기 실패: %w", err)
	}
	privateKey, err := parseRSAPrivateKey(privBytes)
	if err != nil {
		return nil, fmt.Errorf("jwt: private key 파싱 실패: %w", err)
	}

	pubBytes, err := os.ReadFile(cfg.JWTPublicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("jwt: public key 파일 읽기 실패: %w", err)
	}
	publicKey, err := parseRSAPublicKey(pubBytes)
	if err != nil {
		return nil, fmt.Errorf("jwt: public key 파싱 실패: %w", err)
	}

	return &Manager{privateKey: privateKey, publicKey: publicKey}, nil
}

// IssueAccessToken은 userID를 sub로 하는 RS256 JWT를 발급한다.
// 유효기간은 15분이며 iss는 "auth-server"이다.
func (m *Manager) IssueAccessToken(userID uuid.UUID) (string, error) {
	now := time.Now()
	claims := gojwt.RegisteredClaims{
		Subject:   userID.String(),
		IssuedAt:  gojwt.NewNumericDate(now),
		ExpiresAt: gojwt.NewNumericDate(now.Add(accessTokenTTL)),
		Issuer:    issuer,
	}
	token := gojwt.NewWithClaims(gojwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(m.privateKey)
	if err != nil {
		return "", fmt.Errorf("jwt: 토큰 서명 실패: %w", err)
	}
	return signed, nil
}

// ParseAndVerify는 토큰 문자열을 파싱하고 서명·만료를 검증한다.
func (m *Manager) ParseAndVerify(tokenStr string) (*gojwt.RegisteredClaims, error) {
	token, err := gojwt.ParseWithClaims(tokenStr, &gojwt.RegisteredClaims{}, func(t *gojwt.Token) (any, error) {
		if _, ok := t.Method.(*gojwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("jwt: 예상하지 못한 서명 방식: %v", t.Header["alg"])
		}
		return m.publicKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("jwt: 토큰 검증 실패: %w", err)
	}

	claims, ok := token.Claims.(*gojwt.RegisteredClaims)
	if !ok || !token.Valid {
		return nil, fmt.Errorf("jwt: 유효하지 않은 토큰")
	}
	return claims, nil
}

// GenerateOpaqueRefreshToken은 crypto/rand 기반의 불투명 refresh token과
// 그 SHA-256 해시를 반환한다.
func GenerateOpaqueRefreshToken() (raw string, hash string, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", fmt.Errorf("jwt: refresh token 생성 실패: %w", err)
	}
	raw = hex.EncodeToString(b)
	sum := sha256.Sum256([]byte(raw))
	hash = hex.EncodeToString(sum[:])
	return raw, hash, nil
}

func parseRSAPrivateKey(data []byte) (*rsa.PrivateKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("jwt: PEM 블록을 디코딩할 수 없음")
	}
	// PKCS8 우선, 실패 시 PKCS1 시도
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return x509.ParsePKCS1PrivateKey(block.Bytes)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("jwt: RSA 개인키가 아님")
	}
	return rsaKey, nil
}

func parseRSAPublicKey(data []byte) (*rsa.PublicKey, error) {
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, fmt.Errorf("jwt: PEM 블록을 디코딩할 수 없음")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("jwt: 공개키 파싱 실패: %w", err)
	}
	rsaKey, ok := key.(*rsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("jwt: RSA 공개키가 아님")
	}
	return rsaKey, nil
}
