package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/coldair426/auth-server/internal/auth"
	"github.com/coldair426/auth-server/internal/client"
	"github.com/coldair426/auth-server/internal/consent"
	"github.com/coldair426/auth-server/internal/platform/cache"
	"github.com/coldair426/auth-server/internal/platform/config"
	"github.com/coldair426/auth-server/internal/platform/httpx"
	"github.com/coldair426/auth-server/internal/platform/jwt"
	"github.com/coldair426/auth-server/internal/platform/oauth"
	"github.com/coldair426/auth-server/internal/platform/postgres"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("설정 로드 실패", "error", err)
		os.Exit(1)
	}

	// DB 연결
	pool, err := postgres.Connect(context.Background(), cfg.DatabaseURL)
	if err != nil {
		slog.Error("DB 연결 실패", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	// JWT Manager
	jwtMgr, err := jwt.New(cfg)
	if err != nil {
		slog.Error("JWT Manager 생성 실패", "error", err)
		os.Exit(1)
	}

	// sqlc Queries
	q := postgres.New(pool)

	// Repos
	userRepo := postgres.NewUserRepo(q)
	oauthAcctRepo := postgres.NewOAuthAccountRepo(q)
	oauthClientRepo := postgres.NewOAuthClientRepo(q)
	refreshTokenRepo := postgres.NewRefreshTokenRepo(q)
	membershipRepo := postgres.NewMembershipRepo(q)
	consentRepo := postgres.NewConsentRepo(q)

	// Caches
	clientCache := cache.NewClientCache(oauthClientRepo)
	tokenCache := cache.NewRefreshTokenCache(refreshTokenRepo)

	// OAuth Registry
	oauthReg := oauth.NewRegistry(cfg)

	// State store
	stateStore := auth.NewStateStore()

	// Services
	authSvc := auth.NewService(
		userRepo, oauthAcctRepo, clientCache,
		tokenCache, membershipRepo,
		stateStore, jwtMgr, oauthReg,
	)
	clientSvc := client.NewService(clientCache)
	consentSvc := consent.NewService(consentRepo)

	// Handlers
	authHandler := auth.NewHandler(authSvc, cfg.CookieDomain)
	clientHandler := client.NewHandler(clientSvc)
	consentHandler := consent.NewHandler(consentSvc)

	// Router
	r := routes(cfg, jwtMgr, logger, authHandler, clientHandler, consentHandler)

	addr := fmt.Sprintf(":%s", cfg.Port)
	srv := &http.Server{
		Addr:         addr,
		Handler:      r,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("서버 시작", "addr", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("서버 오류", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGTERM, syscall.SIGINT)
	<-quit

	slog.Info("서버 종료 중")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("종료 오류", "error", err)
	}
	slog.Info("서버 종료 완료")
}

func routes(
	cfg *config.Config,
	jwtMgr *jwt.Manager,
	logger *slog.Logger,
	authH *auth.Handler,
	clientH *client.Handler,
	consentH *consent.Handler,
) http.Handler {
	r := chi.NewRouter()

	r.Use(httpx.Recovery(logger))
	r.Use(httpx.RequestLogger(logger))
	r.Use(httpx.CORS(cfg.AllowedOrigins))

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	// 클라이언트
	r.Get("/clients/{clientId}", clientH.GetClient)

	// 인증
	r.Route("/auth", func(r chi.Router) {
		r.Get("/{provider}/url", authH.GetLoginURL)
		r.Post("/{provider}/callback", authH.HandleCallback)
		r.Post("/refresh", authH.Refresh)
		r.Post("/logout", authH.Logout)

		// 인증 필요
		r.Group(func(r chi.Router) {
			r.Use(httpx.AuthMiddleware(jwtMgr))
			r.Post("/join", authH.Join)
		})
	})

	// 동의
	r.Get("/users/{userId}/consents", consentH.ListConsents)
	r.Group(func(r chi.Router) {
		r.Use(httpx.AuthMiddleware(jwtMgr))
		r.Post("/consents", consentH.RecordConsents)
	})

	return r
}
