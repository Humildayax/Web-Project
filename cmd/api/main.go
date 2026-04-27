package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"security-portal/internal/config"
	"security-portal/internal/handlers"
	"security-portal/internal/jira"
	"security-portal/internal/repository"
	"security-portal/internal/services"

	"github.com/go-chi/chi/v5"
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
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	pool, err := setupDB(ctx, cfg)
	if err != nil {
		return err
	}
	defer pool.Close()

	jiraClient := setupJiraClient(cfg)

	deps := wireDependencies(cfg, pool, jiraClient)
	go deps.worker.Run(ctx)

	router := buildRouter(deps)
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
		slog.Info("jira habilitado", "base_url", cfg.Jira.BaseURL, "project", cfg.Jira.ProjectKey)
		return jira.NewHTTPClient(cfg.Jira)
	}
	slog.Warn("jira deshabilitado (modo dev)")
	return jira.NewNoopClient()
}

// dependencies agrupa lo que arma run() y consumen router/worker.
type dependencies struct {
	incidentH *handlers.IncidentHandler
	newsH     *handlers.NewsHandler
	worker    *services.JiraRetryWorker
}

func wireDependencies(cfg config.Config, pool *pgxpool.Pool, jiraClient jira.Client) dependencies {
	incidentRepo := repository.NewIncidentRepository(pool)
	incidentSvc := services.NewIncidentService(incidentRepo, jiraClient)

	newsProvider := repository.NewRSSNewsProvider(cfg.News.FeedURL, cfg.News.Limit)
	newsSvc := services.NewNewsService(newsProvider, cfg.News.CacheTTL)

	return dependencies{
		incidentH: handlers.NewIncidentHandler(incidentSvc, cfg.HTTP.MaxBodyBytes),
		newsH:     handlers.NewNewsHandler(newsSvc),
		worker:    services.NewJiraRetryWorker(incidentRepo, jiraClient, cfg.Jira.RetryInterval, cfg.Jira.MaxRetries),
	}
}

func buildRouter(d dependencies) http.Handler {
	r := chi.NewRouter()
	r.Use(handlers.Recover)
	r.Use(handlers.Logging)

	r.Route("/api", func(api chi.Router) {
		api.Post("/incidents", d.incidentH.CreateIncident)
		api.Get("/news", d.newsH.GetNews)
	})
	return r
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
