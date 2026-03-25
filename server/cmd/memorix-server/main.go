package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/devioslang/memorix/server/internal/config"
	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/embed"
	"github.com/devioslang/memorix/server/internal/handler"
	"github.com/devioslang/memorix/server/internal/llm"
	"github.com/devioslang/memorix/server/internal/middleware"
	"github.com/devioslang/memorix/server/internal/repository/tidb"
	"github.com/devioslang/memorix/server/internal/service"
	"github.com/devioslang/memorix/server/internal/tenant"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if a.Key == slog.TimeKey {
				if t, ok := a.Value.Any().(time.Time); ok {
					a.Value = slog.StringValue(t.Local().Format(time.RFC3339))
				}
			}
			return a
		},
	}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "err", err)
		os.Exit(1)
	}

	db, err := tidb.NewDB(cfg.DSN)
	if err != nil {
		logger.Error("failed to connect database", "err", err)
		os.Exit(1)
	}
	defer db.Close()

	// Embedder (nil if not configured → keyword-only search).
	embedder := embed.New(embed.Config{
		APIKey:  cfg.EmbedAPIKey,
		BaseURL: cfg.EmbedBaseURL,
		Model:   cfg.EmbedModel,
		Dims:    cfg.EmbedDims,
	})
	if cfg.EmbedAutoModel != "" {
		logger.Info("auto-embedding enabled (TiDB EMBED_TEXT)", "model", cfg.EmbedAutoModel, "dims", cfg.EmbedAutoDims)
	} else if embedder != nil {
		logger.Info("client-side embedding configured", "model", cfg.EmbedModel, "dims", cfg.EmbedDims)
	} else {
		logger.Info("no embedding configured, keyword-only search active")
	}
	// LLM client (nil if not configured → raw ingest mode).
	llmClient := llm.New(llm.Config{
		APIKey:      cfg.LLMAPIKey,
		BaseURL:     cfg.LLMBaseURL,
		Model:       cfg.LLMModel,
		Temperature: cfg.LLMTemperature,
	})
	if llmClient != nil {
		logger.Info("LLM configured for smart ingest", "model", cfg.LLMModel)
	} else {
		logger.Info("no LLM configured, ingest will use raw mode")
	}

	// Repositories.
	tenantRepo := tidb.NewTenantRepo(db)
	uploadTaskRepo := tidb.NewUploadTaskRepo(db)
	tenantPool := tenant.NewPool(tenant.PoolConfig{
		MaxIdle:     cfg.TenantPoolMaxIdle,
		MaxOpen:     cfg.TenantPoolMaxOpen,
		IdleTimeout: cfg.TenantPoolIdleTimeout,
		TotalLimit:  cfg.TenantPoolTotalLimit,
	})
	defer tenantPool.Close()

	// Services.
	var zeroClient *tenant.ZeroClient
	if cfg.TiDBZeroEnabled {
		zeroClient = tenant.NewZeroClient(cfg.TiDBZeroAPIURL)
	}
	tenantSvc := service.NewTenantService(tenantRepo, zeroClient, tenantPool, logger, cfg.EmbedAutoModel, cfg.EmbedAutoDims, cfg.FTSEnabled)

	// Middleware.
	tenantMW := middleware.ResolveTenant(tenantRepo, tenantPool)
	rl := middleware.NewRateLimiter(cfg.RateLimit, cfg.RateBurst)
	defer rl.Stop()
	rateMW := rl.Middleware()

	// Metrics collector.
	metricsCollector := service.NewMetricsCollector(logger)

	// Handler.
	gcConfig := domain.GCConfig{
		Enabled:                 cfg.GCEnabled,
		Interval:                cfg.GCInterval,
		StaleThreshold:          cfg.GCStaleThreshold,
		LowConfidenceThreshold:  cfg.GCLowConfidenceThreshold,
		MaxMemoriesPerTenant:    cfg.GCMaxMemoriesPerTenant,
		SnapshotRetentionDays:   cfg.GCSnapshotRetentionDays,
		BatchSize:               cfg.GCBatchSize,
	}
	rulesConfig := domain.RulesConfig{
		OrganizationRulesPath: cfg.RulesOrganizationPath,
		UserRulesPath:         cfg.RulesUserPath,
		EnableOrganization:    cfg.RulesEnabled,
		EnableUser:            cfg.RulesEnabled,
		EnableProject:         cfg.RulesEnabled,
		EnableModule:          cfg.RulesEnabled,
	}
	rulesInjectionConfig := domain.RulesInjectionConfig{
		Enabled:   cfg.RulesInjectionEnabled,
		MaxTokens: cfg.RulesInjectionMaxTokens,
		Header:    cfg.RulesInjectionHeader,
		InjectAt:  "start",
	}
	srv := handler.NewServer(
		tenantSvc,
		uploadTaskRepo,
		cfg.UploadDir,
		embedder,
		llmClient,
		cfg.EmbedAutoModel,
		cfg.FTSEnabled,
		service.IngestMode(cfg.IngestMode),
		logger,
		nil, // vectorStore: not configured by default; set up separately if needed
		cfg.MaxContextTokens,
		cfg.TokenizerType,
		cfg.TokenizerModel,
		cfg.SystemPromptReservedTokens,
		cfg.MemoryReservedTokens,
		cfg.MetadataReservedTokens,
		cfg.UserMemoryBudgetMin,
		cfg.UserMemoryBudgetMax,
		cfg.SummaryBudgetMin,
		cfg.SummaryBudgetMax,
		gcConfig,
		rulesConfig,
		rulesInjectionConfig,
		metricsCollector,
		cfg.DashboardToken,
		tenantRepo,
	)
	router := srv.Router(tenantMW, rateMW)

	httpSrv := &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Upload worker (async file ingest).
	workerCtx, workerCancel := context.WithCancel(context.Background())
	defer workerCancel()
	uploadWorker := service.NewUploadWorker(
		uploadTaskRepo,
		tenantRepo,
		tenantPool,
		embedder,
		llmClient,
		cfg.EmbedAutoModel,
		cfg.FTSEnabled,
		service.IngestMode(cfg.IngestMode),
		logger,
		cfg.WorkerConcurrency,
	)
	go func() {
		if err := uploadWorker.Run(workerCtx); err != nil {
			logger.Error("upload worker error", "err", err)
		}
	}()

	// Graceful shutdown.
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		sig := <-sigCh
		logger.Info("received signal, shutting down", "signal", sig)

		workerCancel() // Stop upload worker first.

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := httpSrv.Shutdown(ctx); err != nil {
			logger.Error("shutdown error", "err", err)
		}
	}()

	logger.Info("starting memorix server", "port", cfg.Port)
	if err := httpSrv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server error", "err", err)
		os.Exit(1)
	}
	logger.Info("server stopped")
}
