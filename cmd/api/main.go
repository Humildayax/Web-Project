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

	"github.com/joho/godotenv"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// --- Postgres ---
	pool, err := repository.NewPool(ctx, cfg.DB)
	if err != nil {
		return err
	}
	defer pool.Close()
	slog.Info("postgres conectado")

	// --- JIRA ---
	var jiraClient jira.Client
	if cfg.Jira.Enabled {
		jiraClient = jira.NewHTTPClient(cfg.Jira)
		slog.Info("jira habilitado", "base_url", cfg.Jira.BaseURL, "project", cfg.Jira.ProjectKey)
	} else {
		jiraClient = jira.NewNoopClient()
		slog.Warn("jira deshabilitado (modo dev)")
	}

	// --- Wiring ---
	incidentRepo := repository.NewIncidentRepository(pool)
	incidentSvc := services.NewIncidentService(incidentRepo, jiraClient)
	incidentH := handlers.NewIncidentHandler(incidentSvc, cfg.HTTP.MaxBodyBytes)

	newsProvider := repository.NewRSSNewsProvider(cfg.News.FeedURL, cfg.News.Limit)
	newsSvc := services.NewNewsService(newsProvider, cfg.News.CacheTTL)
	newsH := handlers.NewNewsHandler(newsSvc)

	worker := services.NewJiraRetryWorker(incidentRepo, jiraClient, cfg.Jira.RetryInterval, cfg.Jira.MaxRetries)
	go worker.Run(ctx)

	// --- HTTP ---
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/incidents", incidentH.CreateIncident)
	mux.HandleFunc("GET /api/news", newsH.GetNews)

	handler := handlers.Chain(mux, handlers.Recover, handlers.Logging)

	srv := &http.Server{
		Addr:              cfg.HTTP.Addr,
		Handler:           handler,
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
