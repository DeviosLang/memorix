package handler

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/devioslang/memorix/server/internal/domain"
	"github.com/devioslang/memorix/server/internal/embed"
	"github.com/devioslang/memorix/server/internal/llm"
	"github.com/devioslang/memorix/server/internal/middleware"
	"github.com/devioslang/memorix/server/internal/repository"
	"github.com/devioslang/memorix/server/internal/repository/tidb"
	"github.com/devioslang/memorix/server/internal/service"
	"github.com/devioslang/memorix/server/internal/tokenizer"
)

// Server holds the HTTP handlers and their dependencies.
type Server struct {
	tenant      *service.TenantService
	uploadTasks repository.UploadTaskRepo
	uploadDir   string
	embedder    *embed.Embedder
	llmClient   *llm.Client
	autoModel   string
	ftsEnabled  bool
	ingestMode  service.IngestMode
	logger      *slog.Logger
	svcCache    sync.Map

	// Context window management
	tokenizer           tokenizer.Tokenizer
	contextWindowConfig service.ContextWindowConfig

	// User profile configuration
	maxFactsPerUser int
}

// NewServer creates a new HTTP handler server.
func NewServer(
	tenantSvc *service.TenantService,
	uploadTasks repository.UploadTaskRepo,
	uploadDir string,
	embedder *embed.Embedder,
	llmClient *llm.Client,
	autoModel string,
	ftsEnabled bool,
	ingestMode service.IngestMode,
	logger *slog.Logger,
	maxContextTokens int,
	tokenizerType string,
	tokenizerModel string,
	systemPromptReservedTokens int,
	memoryReservedTokens int,
) *Server {
	// Create tokenizer based on configuration
	tok, err := tokenizer.New(tokenizer.Config{
		Type:  tokenizer.TokenizerType(tokenizerType),
		Model: tokenizerModel,
	})
	if err != nil {
		logger.Warn("failed to create tokenizer, using default", "err", err)
		tok = tokenizer.NewDefault()
	}

	// Create context window config
	contextConfig := service.ContextWindowConfig{
		MaxTokens:                  maxContextTokens,
		SystemPromptReservedTokens: systemPromptReservedTokens,
		MemoryReservedTokens:       memoryReservedTokens,
		Tokenizer:                  tok,
	}

	return &Server{
		tenant:              tenantSvc,
		uploadTasks:         uploadTasks,
		uploadDir:           uploadDir,
		embedder:            embedder,
		llmClient:           llmClient,
		autoModel:           autoModel,
		ftsEnabled:          ftsEnabled,
		ingestMode:          ingestMode,
		logger:              logger,
		tokenizer:           tok,
		contextWindowConfig: contextConfig,
		maxFactsPerUser:     200, // Default capacity per user
	}
}

// resolvedSvc holds the correct service instances for a request.
// Services are always backed by the tenant's dedicated DB.
type resolvedSvc struct {
	memory      *service.MemoryService
	ingest      *service.IngestService
	userProfile *service.UserProfileService
}

type tenantSvcKey string

// resolveServices returns the correct services for a request.
func (s *Server) resolveServices(auth *domain.AuthInfo) resolvedSvc {
	if auth.TenantID == "" {
		key := tenantSvcKey(fmt.Sprintf("db-%p", auth.TenantDB))
		if cached, ok := s.svcCache.Load(key); ok {
			return cached.(resolvedSvc)
		}
		memRepo := tidb.NewMemoryRepo(auth.TenantDB, s.autoModel, s.ftsEnabled)
		factRepo := tidb.NewUserProfileFactRepo(auth.TenantDB)
		svc := resolvedSvc{
			memory:      service.NewMemoryService(memRepo, s.llmClient, s.embedder, s.autoModel, s.ingestMode),
			ingest:      service.NewIngestService(memRepo, s.llmClient, s.embedder, s.autoModel, s.ingestMode),
			userProfile: service.NewUserProfileService(factRepo, s.maxFactsPerUser),
		}
		s.svcCache.Store(key, svc)
		return svc
	}
	key := tenantSvcKey(fmt.Sprintf("%s-%p", auth.TenantID, auth.TenantDB))
	if cached, ok := s.svcCache.Load(key); ok {
		return cached.(resolvedSvc)
	}
	memRepo := tidb.NewMemoryRepo(auth.TenantDB, s.autoModel, s.ftsEnabled)
	factRepo := tidb.NewUserProfileFactRepo(auth.TenantDB)
	svc := resolvedSvc{
		memory:      service.NewMemoryService(memRepo, s.llmClient, s.embedder, s.autoModel, s.ingestMode),
		ingest:      service.NewIngestService(memRepo, s.llmClient, s.embedder, s.autoModel, s.ingestMode),
		userProfile: service.NewUserProfileService(factRepo, s.maxFactsPerUser),
	}
	s.svcCache.Store(key, svc)
	return svc
}

// resolveUserProfileServices returns the UserProfileService for a request.
func (s *Server) resolveUserProfileServices(auth *domain.AuthInfo) *service.UserProfileService {
	return s.resolveServices(auth).userProfile
}

// Router builds the chi router with all routes and middleware.
func (s *Server) Router(tenantMW, rateLimitMW func(http.Handler) http.Handler) http.Handler {
	r := chi.NewRouter()

	// Global middleware.
	r.Use(chimw.Recoverer)
	r.Use(chimw.RequestID)
	r.Use(requestLogger(s.logger))
	r.Use(rateLimitMW)

	// Health check.
	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		respond(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Provision a new tenant — no auth, no body.
	r.Post("/v1alpha1/memorix", s.provisionMemorix)

	// Tenant-scoped routes — tenantMW resolves {tenantID} to DB connection.
	r.Route("/v1alpha1/memorix/{tenantID}", func(r chi.Router) {
		r.Use(tenantMW)

		// Memory CRUD.
		r.Post("/memories", s.createMemory)
		r.Get("/memories", s.listMemories)
		r.Get("/memories/{id}", s.getMemory)
		r.Put("/memories/{id}", s.updateMemory)
		r.Delete("/memories/{id}", s.deleteMemory)

		// Context window management.
		r.Post("/context", s.contextWindow)
		r.Post("/context/truncate", s.quickTruncate)
		r.Post("/context/count", s.countTokens)

		// Imports (async file ingest).
		r.Post("/imports", s.createTask)
		r.Get("/imports", s.listTasks)
		r.Get("/imports/{id}", s.getTask)

		// User Profile Facts (structured long-term facts about users).
		r.Post("/user-profile/facts", s.createFact)
		r.Get("/user-profile/facts", s.listFacts)
		r.Get("/user-profile/facts/{id}", s.getFact)
		r.Put("/user-profile/facts/{id}", s.updateFact)
		r.Delete("/user-profile/facts/{id}", s.deleteFact)

	})

	return r
}

// respond writes a JSON response.
func respond(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if data != nil {
		if err := json.NewEncoder(w).Encode(data); err != nil {
			slog.Error("failed to encode response", "err", err)
		}
	}
}

// respondError writes a JSON error response.
func respondError(w http.ResponseWriter, status int, msg string) {
	respond(w, status, map[string]string{"error": msg})
}

// handleError maps domain errors to HTTP status codes.
func (s *Server) handleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, domain.ErrNotFound):
		respondError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, domain.ErrWriteConflict):
		respondError(w, http.StatusServiceUnavailable, err.Error())
	case errors.Is(err, domain.ErrConflict):
		respondError(w, http.StatusConflict, err.Error())
	case errors.Is(err, domain.ErrDuplicateKey):
		respondError(w, http.StatusConflict, "duplicate key: "+err.Error())
	case errors.Is(err, domain.ErrValidation):
		respondError(w, http.StatusBadRequest, err.Error())
	default:
		s.logger.Error("internal error", "err", err)
		respondError(w, http.StatusInternalServerError, "internal server error")
	}
}

// decode reads and JSON-decodes the request body.
func decode(r *http.Request, dst any) error {
	if r.Body == nil {
		return &domain.ValidationError{Message: "request body required"}
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(dst); err != nil {
		return &domain.ValidationError{Message: "invalid JSON: " + err.Error()}
	}
	return nil
}

// authInfo extracts AuthInfo from context.
func authInfo(r *http.Request) *domain.AuthInfo {
	return middleware.AuthFromContext(r.Context())
}

// requestLogger returns a middleware that logs each request.
func requestLogger(logger *slog.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			ww := chimw.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)
			logger.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", ww.Status(),
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", chimw.GetReqID(r.Context()),
			)
		})
	}
}
