package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"go.uber.org/zap"

	"github.com/wishmatic/booru-mcp/internal/auth"
	"github.com/wishmatic/booru-mcp/internal/booru"
	"github.com/wishmatic/booru-mcp/internal/catalog"
	"github.com/wishmatic/booru-mcp/internal/config"
	"github.com/wishmatic/booru-mcp/internal/fetch"
	mcpServer "github.com/wishmatic/booru-mcp/internal/mcp"
)

const writeTimeout = 10 * time.Minute

type Server struct {
	cfg       config.Config
	log       *zap.Logger
	router    *chi.Mux
	http      *http.Server
	relations *booru.RelationIndex
	indexCtx  context.Context
	stopIndex context.CancelFunc
}

func New(cfg config.Config, log *zap.Logger) (*Server, error) {
	if cfg.APIKey == "" {
		return nil, auth.ErrNoAPIKey
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	provider, err := buildProvider(cfg, log)
	if err != nil {
		return nil, err
	}

	return newWithProvider(cfg, log, provider, booru.NewRelationIndex(provider, cfg.RelationIndexInterval()))
}

func newWithProvider(
	cfg config.Config,
	log *zap.Logger,
	provider *booru.Client,
	relations *booru.RelationIndex,
) (*Server, error) {
	service := catalog.New(provider, catalog.Options{
		BlockedTags: cfg.BlockedTags(),
		MaxLimit:    cfg.MaxLimit,
		MaxOffset:   cfg.MaxOffset,
		Relations:   relations,
	})

	mcpSrv, err := mcpServer.New(mcpServer.Deps{Log: log, Catalog: service})
	if err != nil {
		return nil, fmt.Errorf("build mcp server: %w", err)
	}

	router := chi.NewRouter()

	router.Use(middleware.RequestID)
	router.Use(middleware.ClientIPFromRemoteAddr)
	router.Use(middleware.Recoverer)
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders:   []string{"Accept", "Content-Type", "Authorization"},
		ExposedHeaders:   []string{},
		AllowCredentials: false,
		MaxAge:           300,
	}))

	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return mcpSrv
	}, nil)

	router.Mount("/mcp", auth.Middleware(log, cfg.APIKey)(handler))

	router.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	indexCtx, stopIndex := context.WithCancel(context.Background())

	return &Server{
		cfg:       cfg,
		log:       log,
		router:    router,
		relations: relations,
		indexCtx:  indexCtx,
		stopIndex: stopIndex,
		http: &http.Server{
			Addr:              cfg.Addr(),
			Handler:           router,
			ReadHeaderTimeout: 10 * time.Second,
			ReadTimeout:       30 * time.Second,
			WriteTimeout:      writeTimeout,
			IdleTimeout:       60 * time.Second,
		},
	}, nil
}

func buildProvider(cfg config.Config, log *zap.Logger) (*booru.Client, error) {
	transport, err := fetch.New(fetch.Config{
		BaseURL:       booru.DefaultBaseURL,
		UserAgent:     cfg.UserAgent,
		Timeout:       time.Duration(cfg.RequestTimeoutSeconds) * time.Second,
		Limiter:       fetch.NewLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst),
		VerboseErrors: cfg.VerboseErrors(),
	})
	if err != nil {
		return nil, fmt.Errorf("danbooru transport: %w", err)
	}

	provider := booru.New(booru.Config{
		BaseURL:  booru.DefaultBaseURL,
		Login:    cfg.DanbooruLogin,
		APIKey:   cfg.DanbooruAPIKey,
		MaxLimit: cfg.MaxLimit,
		HTTP:     transport,
	})

	log.Info("client enabled",
		zap.String("client", provider.Name()),
		zap.String("base_url", booru.DefaultBaseURL),
		zap.Bool("authenticated", cfg.DanbooruAuthenticated()),
	)

	return provider, nil
}

func (s *Server) Run() error {
	s.startRelationIndex()

	s.log.Info("server listening", zap.String("addr", s.cfg.Addr()))

	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

// startRelationIndex crawls the canonical alias and implication graphs in the background. It is started here rather
// than in New so that constructing a server never makes a network call, which keeps tests and offline use quiet.
func (s *Server) startRelationIndex() {
	if s.relations == nil {
		return
	}

	s.relations.Start(s.indexCtx, func(err error) {
		s.log.Warn("relation index refresh failed", zap.Error(err))
	})
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.stopIndex != nil {
		s.stopIndex()
	}

	return s.http.Shutdown(ctx)
}
