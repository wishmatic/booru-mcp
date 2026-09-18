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
	"github.com/wishmatic/booru-mcp/internal/danbooru"
	"github.com/wishmatic/booru-mcp/internal/fetch"
	"github.com/wishmatic/booru-mcp/internal/gelbooru"
	mcpServer "github.com/wishmatic/booru-mcp/internal/mcp"
	"github.com/wishmatic/booru-mcp/internal/moebooru"
	"github.com/wishmatic/booru-mcp/internal/store"
)

const writeTimeout = 10 * time.Minute

type Server struct {
	cfg      config.Config
	log      *zap.Logger
	store    *store.Client
	registry *booru.Registry
	router   *chi.Mux
	http     *http.Server
}

func New(cfg config.Config, log *zap.Logger) (*Server, error) {
	if cfg.APIKey == "" {
		return nil, auth.ErrNoAPIKey
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	storeClient, err := store.New(cfg.DBPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if mode := storeClient.JournalMode(); mode != "wal" {
		log.Warn("sqlite is not in WAL mode; this filesystem does not support it",
			zap.String("path", cfg.DBPath),
			zap.String("journal_mode", mode),
		)
	}

	log.Info("database opened",
		zap.String("path", cfg.DBPath),
		zap.String("journal_mode", storeClient.JournalMode()),
		zap.Int("cache_ttl_days", cfg.CacheTTLDays),
	)

	registry, relatedClients, err := buildClients(cfg, log)
	if err != nil {
		_ = storeClient.Close()

		return nil, err
	}

	if len(registry.Active()) == 0 {
		log.Warn("no clients are active; every tool will return empty results explaining why")
	}

	contentRating, err := cfg.ContentRating()
	if err != nil {
		_ = storeClient.Close()

		return nil, err
	}

	catalogService := catalog.New(registry, storeClient, catalog.Options{
		DefaultClients: cfg.DefaultClients(),
		CacheTTL:       cfg.CacheTTL(),
		ContentRating:  contentRating,
		BlockedTags:    cfg.BlockedTags(),
		MaxLimit:       cfg.MaxLimit,
	})

	mcpSrv, err := mcpServer.New(mcpServer.Deps{
		Log:                   log,
		Catalog:               catalogService,
		RelatedClients:        relatedClients,
		RelatedDefaultClients: intersect(relatedClients, cfg.DefaultClients()),
	})
	if err != nil {
		_ = storeClient.Close()

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

	return &Server{
		cfg:      cfg,
		log:      log,
		store:    storeClient,
		registry: registry,
		router:   router,
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

func buildClients(cfg config.Config, log *zap.Logger) (*booru.Registry, []string, error) {
	registry := booru.NewRegistry()
	limiter := fetch.NewLimiter(cfg.RateLimitRPS, cfg.RateLimitBurst)

	var related []string

	for _, name := range cfg.EnabledClients() {
		spec, _ := config.ClientSpecByName(name)

		active, reason := cfg.ClientActive(name)
		if !active {
			if err := registry.Register(name, nil, reason); err != nil {
				return nil, nil, err
			}

			log.Info("client inactive", zap.String("client", name), zap.String("reason", reason))

			continue
		}

		transport, err := fetch.New(fetch.Config{
			BaseURL:       cfg.ClientURL(name),
			UserAgent:     cfg.UserAgent,
			Timeout:       time.Duration(cfg.RequestTimeoutSeconds) * time.Second,
			Limiter:       limiter,
			VerboseErrors: cfg.VerboseErrors(),
		})
		if err != nil {
			return nil, nil, fmt.Errorf("client %s: %w", name, err)
		}

		provider, err := buildProvider(spec, cfg, transport)
		if err != nil {
			return nil, nil, err
		}

		if err := registry.Register(name, provider, ""); err != nil {
			return nil, nil, err
		}

		if _, ok := provider.(booru.RelatedTagProvider); ok {
			related = append(related, name)
		}

		log.Info("client enabled", zap.String("client", name), zap.String("family", string(spec.Family)))
	}

	return registry, related, nil
}

func buildProvider(spec config.ClientSpec, cfg config.Config, transport *fetch.Client) (booru.Provider, error) {
	cred := func(envs []string, index int) string {
		if index >= len(envs) {
			return ""
		}

		return cfg.Env(envs[index])
	}

	switch spec.Family {
	case config.FamilyDanbooru:
		return danbooru.New(danbooru.Config{
			Name:     spec.Name,
			BaseURL:  cfg.ClientURL(spec.Name),
			Login:    cred(spec.OptionalCreds, 0),
			APIKey:   cred(spec.OptionalCreds, 1),
			MaxLimit: cfg.MaxLimit,
			HTTP:     transport,
		}), nil
	case config.FamilyGelbooru:
		return gelbooru.New(gelbooru.Config{
			Name:           spec.Name,
			BaseURL:        cfg.ClientURL(spec.Name),
			APIKey:         cred(spec.RequiredCreds, 0),
			UserID:         cred(spec.RequiredCreds, 1),
			MaxLimit:       cfg.MaxLimit,
			CredentialEnvs: spec.RequiredCreds,
			HTTP:           transport,
		}), nil
	case config.FamilyMoebooru:
		return moebooru.New(moebooru.Config{
			Name:     spec.Name,
			BaseURL:  cfg.ClientURL(spec.Name),
			MaxLimit: cfg.MaxLimit,
			HTTP:     transport,
		}), nil
	default:
		return nil, fmt.Errorf("client %s: unsupported family %q", spec.Name, spec.Family)
	}
}

func intersect(names, allowed []string) []string {
	allowedSet := make(map[string]bool, len(allowed))
	for _, name := range allowed {
		allowedSet[name] = true
	}

	var out []string

	for _, name := range names {
		if allowedSet[name] {
			out = append(out, name)
		}
	}

	return out
}

func (s *Server) Run() error {
	s.log.Info("server listening", zap.String("addr", s.cfg.Addr()))

	if err := s.http.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}

	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	return errors.Join(s.http.Shutdown(ctx), s.store.Close())
}
