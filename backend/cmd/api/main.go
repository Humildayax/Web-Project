package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"security-portal/internal/config"
	"security-portal/internal/handlers"
	"security-portal/internal/jira"
	migrator "security-portal/internal/migrate"
	"security-portal/internal/news"
	"security-portal/internal/repository"
	"security-portal/internal/services"
	"security-portal/internal/storage"
	"security-portal/migrations"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	"github.com/go-chi/httprate"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
)

func main() {
	setupLogger()
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

// ---- bootstrap dividido en pasos pequeños ----

func setupLogger() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})))
}

func run() error {
	// Buscamos .env en CWD primero, después en el directorio padre.
	// Esto cubre: `go run ./cmd/api` desde backend/ (CWD=backend, .env vive en ../)
	// y también el caso de tener un .env local en backend/ si alguien lo prefiere.
	_ = godotenv.Load()
	_ = godotenv.Load("../.env")

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	if cfg.DB.MigrateOnStart {
		if err := migrator.Up(ctx, cfg.DB.DSN, migrations.FS); err != nil {
			return err
		}
	} else {
		slog.Info("migraciones deshabilitadas en arranque (MIGRATE_ON_START=false)")
	}

	pool, err := setupDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	jiraClient := setupJiraClient(cfg)

	store, err := storage.NewLocal(cfg.Storage.BasePath)
	if err != nil {
		return fmt.Errorf("storage: %w", err)
	}
	slog.Info("storage local listo", "base_path", cfg.Storage.BasePath)

	deps := wireDependencies(cfg, pool, jiraClient, store)
	go deps.jiraWorker.Run(ctx)
	if deps.retentionWorker != nil {
		go deps.retentionWorker.Run(ctx)
	}

	router := buildRouter(deps, cfg.HTTP, pool)
	return runServer(ctx, cfg, router)
}

func setupDB(ctx context.Context, cfg config.Config) (*pgxpool.Pool, error) {
	pool, err := repository.NewPool(ctx, cfg.DB)
	if err != nil {
		return nil, err
	}
	slog.Info("postgres conectado")
	return pool, nil
}

func setupJiraClient(cfg config.Config) jira.Client {
	if cfg.Jira.Enabled {
		slog.Info("jira habilitado",
			"base_url", cfg.Jira.BaseURL,
			"project", cfg.Jira.ProjectKey,
			"issue_type", cfg.Jira.IssueType,
			"optional_fields", configuredJiraOptionals(cfg.Jira),
		)
		return jira.NewHTTPClient(cfg.Jira)
	}
	slog.Warn("jira deshabilitado (modo dev)")
	return jira.NewNoopClient()
}

// configuredJiraOptionals lista las KEYS de los campos opcionales que el
// cliente JIRA va a aplicar via PUT post-create. Si la lista llega vacía
// al log de arranque, las variables del .env no están llegando al
// contenedor (chequear docker-compose environment).
func configuredJiraOptionals(j config.JiraConfig) []string {
	var out []string
	if j.AssigneeAccountID != "" {
		out = append(out, "assignee")
	}
	if j.ReporterAccountID != "" {
		out = append(out, "reporter")
	}
	if j.PriorityName != "" {
		out = append(out, "priority")
	}
	if j.ParentID != "" {
		out = append(out, "parent")
	}
	if j.EpicLinkFieldID != "" && j.EpicLinkValue != "" {
		out = append(out, "epic_link")
	}
	if j.StartDateFieldID != "" {
		out = append(out, "start_date")
	}
	return out
}

// dependencies agrupa lo que arma run() y consumen router/workers.
type dependencies struct {
	incidentH       *handlers.IncidentHandler
	newsH           *handlers.NewsHandler
	jiraWorker      *services.JiraRetryWorker
	retentionWorker *services.RetentionWorker // nil si retention deshabilitado
}

func wireDependencies(cfg config.Config, pool *pgxpool.Pool, jiraClient jira.Client, store storage.Storage) dependencies {
	incidentRepo := repository.NewIncidentRepository(pool, cfg.DB.OpTimeout)
	attachmentRepo := repository.NewAttachmentRepository(pool, cfg.DB.OpTimeout)
	incidentSvc := services.NewIncidentService(incidentRepo, attachmentRepo, store, jiraClient)

	newsProvider := news.NewRSSProvider(cfg.News.FeedURLs, cfg.News.LimitPerSource)
	newsSvc := services.NewNewsService(newsProvider, cfg.News.CacheTTL)

	deps := dependencies{
		incidentH: handlers.NewIncidentHandler(
			incidentSvc,
			cfg.HTTP.MaxBodyBytes,
			handlers.AttachmentLimits{
				MaxFiles:       cfg.Attachment.MaxFiles,
				MaxFileBytes:   cfg.Attachment.MaxFileBytes,
				MaxImageDim:    cfg.Attachment.MaxImageDim,
				MaxMemoryParse: cfg.Attachment.MaxMemoryParse,
			},
		),
		newsH: handlers.NewNewsHandler(newsSvc),
		jiraWorker: services.NewJiraRetryWorker(
			incidentRepo,
			attachmentRepo,
			store,
			jiraClient,
			cfg.Jira.RetryInterval,
			cfg.Jira.BaseBackoff,
			cfg.Jira.MaxRetries,
			cfg.Jira.AttachmentBatchSize,
		),
	}
	if cfg.Retention.IncidentMetadataMaxAge > 0 {
		deps.retentionWorker = services.NewRetentionWorker(
			incidentRepo,
			attachmentRepo,
			store,
			cfg.Retention.IncidentMetadataMaxAge,
			cfg.Retention.Interval,
		)
	} else {
		slog.Warn("retention worker deshabilitado (INCIDENT_METADATA_MAX_AGE <= 0)")
	}
	return deps
}

func buildRouter(d dependencies, httpCfg config.HTTPConfig, pool *pgxpool.Pool) http.Handler {
	r := chi.NewRouter()

	// Orden de middlewares (outer -> inner):
	//   RequestID : asigna un ID único por request, accesible vía
	//               middleware.GetReqID(ctx).
	//   CORS      : atiende preflight OPTIONS y agrega headers a toda
	//               respuesta. Solo se registra si hay orígenes configurados;
	//               en prod, mismo origen no requiere CORS.
	//   Logging   : registra todas las requests (incluido OPTIONS) y propaga
	//               el RequestID al header X-Request-Id de la respuesta.
	//   Recover   : captura panics del handler para que Logging logre loggear.
	r.Use(middleware.RequestID)
	if len(httpCfg.AllowedOrigins) > 0 {
		r.Use(cors.Handler(cors.Options{
			AllowedOrigins:   httpCfg.AllowedOrigins,
			AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
			AllowedHeaders:   []string{"Accept", "Content-Type"},
			ExposedHeaders:   []string{"X-Request-Id", "X-Cache"},
			AllowCredentials: false,
			MaxAge:           300, // segundos que el browser cachea el preflight
		}))
	} else {
		slog.Info("CORS deshabilitado (ALLOWED_ORIGINS vacío)")
	}
	r.Use(handlers.Logging)
	r.Use(handlers.Recover)

	// Rate-limit por IP solo para el POST de incidentes. Usa el mismo
	// criterio de IP del audit trail (X-Real-IP -> último XFF -> RemoteAddr)
	// para que no se pueda evadir falsificando headers desde el cliente.
	incidentLimiter := httprate.Limit(
		httpCfg.IncidentRateLimit,
		httpCfg.IncidentRateWindow,
		httprate.WithKeyFuncs(func(r *http.Request) (string, error) {
			return handlers.ClientIP(r), nil
		}),
	)

	r.Route("/api", func(api chi.Router) {
		// /health = liveness. Responde 200 si el proceso está vivo. No depende
		// de DB ni de servicios externos: si esto falla, el contenedor está
		// muerto y hay que reiniciarlo.
		api.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"status":"ok"}`))
		})
		// /ready = readiness. 200 si la DB responde, 503 si no. Lo que
		// usan k8s/load balancers para sacar la instancia del pool sin
		// matarla.
		api.Get("/ready", readinessHandler(pool))
		api.With(incidentLimiter).Post("/incidents", d.incidentH.CreateIncident)
		api.Get("/news", d.newsH.GetNews)
	})
	return r
}

func readinessHandler(pool *pgxpool.Pool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		if err := pool.Ping(ctx); err != nil {
			slog.Warn("readiness: ping a DB falló", "err", err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"status":"not_ready","reason":"database"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"ready"}`))
	}
}

func runServer(ctx context.Context, cfg config.Config, h http.Handler) error {
	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           h,
		ReadTimeout:       cfg.HTTP.ReadTimeout,
		ReadHeaderTimeout: cfg.HTTP.ReadHeaderTimeout,
		WriteTimeout:      cfg.HTTP.WriteTimeout,
		IdleTimeout:       cfg.HTTP.IdleTimeout,
	}

	errCh := make(chan error, 1)
	go func() {
		slog.Info("servidor http escuchando", "addr", cfg.HTTP.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		slog.Info("señal recibida, apagando...")
	case err := <-errCh:
		return err
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), cfg.HTTP.ShutdownTimeout)
	defer cancel()
	return srv.Shutdown(shutdownCtx)
}
