// BorrowTime — сервис безопасного обмена конфиденциальными данными
// Точка входа: инициализация зависимостей, запуск HTTP-сервера и планировщика

// @title           BorrowTime API
// @version         1.0
// @description     Сервис безопасной одноразовой передачи конфиденциальных файлов.
// @termsOfService  http://swagger.io/terms/

// @contact.name   BorrowTime Team

// @host      localhost:8080
// @BasePath  /api/v1

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Введите токен в формате: Bearer {token}
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	httpSwagger "github.com/swaggo/http-swagger/v2"

	_ "github.com/borrowtime/server/docs"
	"github.com/borrowtime/server/internal/config"
	"github.com/borrowtime/server/internal/handler"
	"github.com/borrowtime/server/internal/postgres"
	"github.com/borrowtime/server/internal/usecase"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		logger.Error("config error", "error", err)
		os.Exit(1)
	}

	if err := postgres.RunMigrations(cfg.Postgres.DSN, logger); err != nil {
		logger.Error("migrations failed", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	pool, err := postgres.NewPool(ctx, cfg.Postgres.DSN)
	if err != nil {
		logger.Error("postgres connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()
	logger.Info("postgres connected")

	transferRepo := postgres.NewTransferRepo(pool)
	auditRepo := postgres.NewAuditRepo(pool)
	settingsRepo := postgres.NewSettingsRepo(pool)
	statsProvider := postgres.NewStatsProvider(pool)
	userRepo := postgres.NewUserRepo(pool)

	storageProvider := &stubStorage{}

	notifier := &stubNotifier{}

	authUC := usecase.NewAuthUseCase(
		userRepo, auditRepo,
		cfg.JWT.Secret, cfg.JWT.AccessTTL, cfg.JWT.RefreshTTL, cfg.JWT.PartialTTL,
		logger,
	)
	createUC := usecase.NewCreateTransfer(transferRepo, auditRepo, storageProvider, settingsRepo, cfg.App.BaseURL)
	getUC := usecase.NewGetFile(transferRepo, auditRepo, storageProvider)
	revokeUC := usecase.NewRevokeAccess(transferRepo, auditRepo, storageProvider, logger)
	lifecycleUC := usecase.NewLifecycle(transferRepo, auditRepo, storageProvider, notifier, logger)
	auditUC := usecase.NewAuditLog(auditRepo)
	exportUC := usecase.NewExportAudit(auditRepo)
	settingsUC := usecase.NewGlobalSettings(settingsRepo, statsProvider)

	go lifecycleUC.Run(ctx, 5*time.Minute)

	r := chi.NewRouter()
	r.Use(handler.CORSMiddleware)
	r.Use(middleware.RealIP)
	r.Use(middleware.RequestID)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Timeout(30 * time.Second))

	r.Route("/api/v1", func(r chi.Router) {
		// JWT middleware — устанавливает userID/role в контекст если токен есть
		r.Use(handler.JWTMiddleware(authUC))

		authH := handler.NewAuthHandler(authUC)
		transferH := handler.NewTransferHandler(createUC, getUC, revokeUC, userRepo)
		auditH := handler.NewAuditHandler(auditUC, exportUC)
		adminH := handler.NewAdminHandler(settingsUC)
		userH := handler.NewUserHandler(userRepo, transferRepo)

		authH.RegisterRoutes(r)
		transferH.RegisterRoutes(r)
		auditH.RegisterRoutes(r)
		adminH.RegisterRoutes(r)
		userH.RegisterRoutes(r)
	})

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		if err := pool.Ping(r.Context()); err != nil {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	// Swagger UI — доступен по /swagger/index.html
	r.Get("/swagger/*", httpSwagger.Handler(
		httpSwagger.URL("/swagger/doc.json"),
	))

	srv := &http.Server{
		Addr:         cfg.HTTP.Addr,
		Handler:      r,
		ReadTimeout:  cfg.HTTP.ReadTimeout,
		WriteTimeout: cfg.HTTP.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	go func() {
		<-ctx.Done()
		shutCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()
		if err := srv.Shutdown(shutCtx); err != nil {
			logger.Error("server shutdown error", "error", err)
		}
	}()

	logger.Info("BorrowTime server starting", "addr", cfg.HTTP.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped")
}
