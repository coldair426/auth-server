package httpx

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/coldair426/auth-server/internal/domain"
	"github.com/coldair426/auth-server/internal/platform/jwt"
	"github.com/google/uuid"
)

// ─── JSON 응답 헬퍼 ──────────────────────────────────────────────────────────

// ErrorEnvelope는 JSON 오류 응답 형식이다.
type ErrorEnvelope struct {
	Message string `json:"message"`
	Code    string `json:"code"`
}

// WriteJSON은 Content-Type을 설정하고 JSON 응답을 작성한다.
func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// WriteError는 ErrorEnvelope 형식의 오류 응답을 작성한다.
func WriteError(w http.ResponseWriter, status int, message, code string) {
	WriteJSON(w, status, ErrorEnvelope{Message: message, Code: code})
}

// DomainError는 도메인 sentinel 오류를 적절한 HTTP 상태 코드로 매핑한다.
func DomainError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrClientNotFound),
		errors.Is(err, domain.ErrUserNotFound),
		errors.Is(err, domain.ErrOAuthAccountNotFound),
		errors.Is(err, domain.ErrMembershipNotFound),
		errors.Is(err, domain.ErrConsentNotFound):
		WriteError(w, http.StatusNotFound, err.Error(), "NOT_FOUND")

	case errors.Is(err, domain.ErrInvalidRedirectURI),
		errors.Is(err, domain.ErrInvalidProvider),
		errors.Is(err, domain.ErrInvalidPolicyType):
		WriteError(w, http.StatusBadRequest, err.Error(), "INVALID_REQUEST")

	case errors.Is(err, domain.ErrRefreshTokenNotFound),
		errors.Is(err, domain.ErrRefreshTokenRevoked),
		errors.Is(err, domain.ErrRefreshTokenExpired):
		WriteError(w, http.StatusUnauthorized, err.Error(), "UNAUTHORIZED")

	default:
		slog.Error("내부 서버 오류", "error", err)
		WriteError(w, http.StatusInternalServerError, "internal server error", "INTERNAL_ERROR")
	}
}

// ─── 미들웨어 ────────────────────────────────────────────────────────────────

// statusWriter는 응답 상태 코드를 캡처하는 ResponseWriter 래퍼이다.
type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *statusWriter) Write(b []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	return w.ResponseWriter.Write(b)
}

// Recovery는 패닉을 복구하고 500을 반환하는 미들웨어이다.
func Recovery(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if rec := recover(); rec != nil {
					logger.Error("패닉 복구", "recover", rec, "path", r.URL.Path)
					WriteError(w, http.StatusInternalServerError, "internal server error", "INTERNAL_ERROR")
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// RequestLogger는 구조화 로그로 요청을 기록하는 미들웨어이다.
func RequestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(sw, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", sw.status,
				"duration_ms", time.Since(start).Milliseconds(),
			)
		})
	}
}

// CORS는 withCredentials 쿠키에 필요한 CORS 헤더를 설정하는 미들웨어이다.
// credentials 허용을 위해 와일드카드(*) 대신 특정 Origin만 허용한다.
func CORS(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			if _, ok := allowed[origin]; ok {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Credentials", "true")
				w.Header().Set("Vary", "Origin")
			}

			if r.Method == http.MethodOptions {
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Max-Age", "86400")
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// ─── 인증 미들웨어 ────────────────────────────────────────────────────────────

type contextKey string

const userIDKey contextKey = "userID"

// AuthMiddleware는 Authorization Bearer 헤더 또는 access_token 쿠키에서
// JWT를 읽어 검증하고, 유효하면 userID를 컨텍스트에 주입한다.
func AuthMiddleware(jwtMgr *jwt.Manager) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tokenStr := bearerToken(r)
			if tokenStr == "" {
				if c, err := r.Cookie("access_token"); err == nil {
					tokenStr = c.Value
				}
			}
			if tokenStr == "" {
				WriteError(w, http.StatusUnauthorized, "인증 정보가 없습니다", "UNAUTHORIZED")
				return
			}

			claims, err := jwtMgr.ParseAndVerify(tokenStr)
			if err != nil {
				WriteError(w, http.StatusUnauthorized, "유효하지 않은 토큰", "UNAUTHORIZED")
				return
			}

			userID, err := uuid.Parse(claims.Subject)
			if err != nil {
				WriteError(w, http.StatusUnauthorized, "유효하지 않은 토큰", "UNAUTHORIZED")
				return
			}

			ctx := context.WithValue(r.Context(), userIDKey, userID)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext는 컨텍스트에서 userID를 추출한다.
func UserIDFromContext(ctx context.Context) (uuid.UUID, bool) {
	id, ok := ctx.Value(userIDKey).(uuid.UUID)
	return id, ok
}

func bearerToken(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	return ""
}

// ─── 쿠키 헬퍼 ───────────────────────────────────────────────────────────────

const (
	accessTokenCookieName = "access_token"
	refreshCookieName     = "refresh"
	accessTokenMaxAge     = 15 * 60      // 15분
	refreshMaxAge         = 30 * 24 * 60 * 60 // 30일
)

// SetAccessTokenCookie는 access_token 쿠키를 설정한다.
func SetAccessTokenCookie(w http.ResponseWriter, token, domain string) {
	http.SetCookie(w, &http.Cookie{
		Name:     accessTokenCookieName,
		Value:    token,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		Path:     "/",
		Domain:   domain,
		MaxAge:   accessTokenMaxAge,
	})
}

// SetRefreshCookie는 refresh 쿠키를 설정한다.
// Path=/auth 로 한정하여 불필요한 요청에 토큰이 노출되지 않도록 한다.
func SetRefreshCookie(w http.ResponseWriter, token, domain string) {
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    token,
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		Path:     "/auth",
		Domain:   domain,
		MaxAge:   refreshMaxAge,
	})
}

// ClearAuthCookies는 access_token과 refresh 쿠키를 삭제한다.
func ClearAuthCookies(w http.ResponseWriter, domain string) {
	http.SetCookie(w, &http.Cookie{
		Name:     accessTokenCookieName,
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		Path:     "/",
		Domain:   domain,
		MaxAge:   -1,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     refreshCookieName,
		Value:    "",
		HttpOnly: true,
		Secure:   true,
		SameSite: http.SameSiteNoneMode,
		Path:     "/auth",
		Domain:   domain,
		MaxAge:   -1,
	})
}
