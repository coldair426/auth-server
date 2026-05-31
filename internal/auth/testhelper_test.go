package auth_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"os"
	"testing"

	"github.com/coldair426/auth-server/internal/platform/config"
	"github.com/coldair426/auth-server/internal/platform/jwt"
)

// newTestJWTManager는 테스트용 RSA 키 쌍을 생성하고 JWT Manager를 반환한다.
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
