package jwt_test

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"os"
	"testing"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"github.com/coldair426/auth-server/internal/platform/config"
	"github.com/coldair426/auth-server/internal/platform/jwt"
)

// setupKeys는 테스트용 RSA 키 쌍을 임시 파일에 저장한다.
func setupKeys(t *testing.T) (privatePath, publicPath string) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("RSA 키 생성 실패: %v", err)
	}

	dir := t.TempDir()

	privFile, err := os.CreateTemp(dir, "private*.pem")
	if err != nil {
		t.Fatalf("임시 파일 생성 실패: %v", err)
	}
	privBytes, _ := x509.MarshalPKCS8PrivateKey(privateKey)
	if err := pem.Encode(privFile, &pem.Block{Type: "PRIVATE KEY", Bytes: privBytes}); err != nil {
		t.Fatalf("private key PEM 인코딩 실패: %v", err)
	}
	privFile.Close()

	pubFile, err := os.CreateTemp(dir, "public*.pem")
	if err != nil {
		t.Fatalf("임시 파일 생성 실패: %v", err)
	}
	pubBytes, _ := x509.MarshalPKIXPublicKey(&privateKey.PublicKey)
	if err := pem.Encode(pubFile, &pem.Block{Type: "PUBLIC KEY", Bytes: pubBytes}); err != nil {
		t.Fatalf("public key PEM 인코딩 실패: %v", err)
	}
	pubFile.Close()

	return privFile.Name(), pubFile.Name()
}

func newManager(t *testing.T) (*jwt.Manager, string, string) {
	t.Helper()
	privatePath, publicPath := setupKeys(t)
	cfg := &config.Config{
		JWTPrivateKeyPath: privatePath,
		JWTPublicKeyPath:  publicPath,
	}
	m, err := jwt.New(cfg)
	if err != nil {
		t.Fatalf("JWT Manager 생성 실패: %v", err)
	}
	return m, privatePath, publicPath
}

func TestIssueAndVerify_Roundtrip(t *testing.T) {
	m, _, _ := newManager(t)
	userID := uuid.MustParse("01900000-0000-7000-0000-000000000001")

	token, err := m.IssueAccessToken(userID)
	if err != nil {
		t.Fatalf("토큰 발급 실패: %v", err)
	}

	claims, err := m.ParseAndVerify(token)
	if err != nil {
		t.Fatalf("토큰 검증 실패: %v", err)
	}

	tests := []struct {
		name string
		got  string
		want string
	}{
		{"sub", claims.Subject, userID.String()},
		{"iss", claims.Issuer, "auth-server"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Errorf("got %q, want %q", tt.got, tt.want)
			}
		})
	}

	t.Run("exp=iat+15m", func(t *testing.T) {
		diff := claims.ExpiresAt.Time.Sub(claims.IssuedAt.Time)
		if diff != 15*time.Minute {
			t.Errorf("유효기간 불일치: got %v, want 15m", diff)
		}
	})
}

func TestParseAndVerify_ExpiredToken(t *testing.T) {
	m, privatePath, _ := newManager(t)

	// 테스트용 만료 토큰을 직접 생성한다
	privBytes, _ := os.ReadFile(privatePath)
	block, _ := pem.Decode(privBytes)
	keyIface, _ := x509.ParsePKCS8PrivateKey(block.Bytes)
	privateKey := keyIface.(*rsa.PrivateKey)

	claims := gojwt.RegisteredClaims{
		Subject:   uuid.New().String(),
		IssuedAt:  gojwt.NewNumericDate(time.Now().Add(-30 * time.Minute)),
		ExpiresAt: gojwt.NewNumericDate(time.Now().Add(-1 * time.Minute)),
		Issuer:    "auth-server",
	}
	raw := gojwt.NewWithClaims(gojwt.SigningMethodRS256, claims)
	expired, err := raw.SignedString(privateKey)
	if err != nil {
		t.Fatalf("만료 토큰 생성 실패: %v", err)
	}

	_, err = m.ParseAndVerify(expired)
	if err == nil {
		t.Error("만료된 토큰이 유효하게 처리됨")
	}
}

func TestParseAndVerify_TamperedToken(t *testing.T) {
	m, _, _ := newManager(t)

	token, _ := m.IssueAccessToken(uuid.New())
	// 서명 부분 변조
	tampered := token[:len(token)-4] + "XXXX"

	_, err := m.ParseAndVerify(tampered)
	if err == nil {
		t.Error("변조된 토큰이 유효하게 처리됨")
	}
}

func TestGenerateOpaqueRefreshToken(t *testing.T) {
	raw, hash, err := jwt.GenerateOpaqueRefreshToken()
	if err != nil {
		t.Fatalf("Refresh Token 생성 실패: %v", err)
	}

	t.Run("비어있지 않음", func(t *testing.T) {
		if raw == "" || hash == "" {
			t.Error("raw 또는 hash가 비어있음")
		}
	})

	t.Run("raw와 hash 불일치", func(t *testing.T) {
		if raw == hash {
			t.Error("raw와 hash가 동일함")
		}
	})

	t.Run("SHA-256 해시 일치", func(t *testing.T) {
		sum := sha256.Sum256([]byte(raw))
		want := hex.EncodeToString(sum[:])
		if hash != want {
			t.Errorf("hash 불일치: got %q, want %q", hash, want)
		}
	})

	t.Run("호출마다 다른 값", func(t *testing.T) {
		raw2, hash2, err := jwt.GenerateOpaqueRefreshToken()
		if err != nil {
			t.Fatalf("두 번째 생성 실패: %v", err)
		}
		if raw == raw2 || hash == hash2 {
			t.Error("연속 호출에서 동일한 값이 반환됨")
		}
	})
}
