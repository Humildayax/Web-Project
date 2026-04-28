package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	HTTP HTTPConfig
	DB   DBConfig
	Jira JiraConfig
	News NewsConfig
}

type HTTPConfig struct {
	Addr              string
	ReadTimeout       time.Duration
	ReadHeaderTimeout time.Duration
	WriteTimeout      time.Duration
	IdleTimeout       time.Duration
	ShutdownTimeout   time.Duration
	MaxBodyBytes      int64
	AllowedOrigins    []string
}

type DBConfig struct {
	DSN               string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration
}

type JiraConfig struct {
	Enabled       bool
	BaseURL       string
	Email         string
	APIToken      string
	ProjectKey    string
	IssueType     string
	HTTPTimeout   time.Duration
	RetryInterval time.Duration
	MaxRetries    int
}

type NewsConfig struct {
	FeedURL  string
	Limit    int
	CacheTTL time.Duration
}

func Load() (Config, error) {
	dsn, err := buildDSN()
	if err != nil {
		return Config{}, err
	}

	cfg := Config{
		HTTP: HTTPConfig{
			Addr:              getenv("HTTP_ADDR", ":8080"),
			ReadTimeout:       getDuration("HTTP_READ_TIMEOUT", 10*time.Second),
			ReadHeaderTimeout: getDuration("HTTP_READ_HEADER_TIMEOUT", 5*time.Second),
			WriteTimeout:      getDuration("HTTP_WRITE_TIMEOUT", 15*time.Second),
			IdleTimeout:       getDuration("HTTP_IDLE_TIMEOUT", 60*time.Second),
			ShutdownTimeout:   getDuration("HTTP_SHUTDOWN_TIMEOUT", 10*time.Second),
			MaxBodyBytes:      int64(getInt("HTTP_MAX_BODY_BYTES", 1<<20)),
			AllowedOrigins:    parseCSV(getenv("ALLOWED_ORIGINS", "http://localhost:5173")),
		},
		DB: DBConfig{
			DSN:               dsn,
			MaxConns:          int32(getInt("DB_MAX_CONNS", 10)),
			MinConns:          int32(getInt("DB_MIN_CONNS", 2)),
			MaxConnLifetime:   getDuration("DB_MAX_CONN_LIFETIME", time.Hour),
			MaxConnIdleTime:   getDuration("DB_MAX_CONN_IDLE", 30*time.Minute),
			HealthCheckPeriod: getDuration("DB_HEALTHCHECK_PERIOD", time.Minute),
		},
		Jira: JiraConfig{
			Enabled:       getBool("JIRA_ENABLED", false),
			BaseURL:       os.Getenv("JIRA_BASE_URL"),
			Email:         os.Getenv("JIRA_EMAIL"),
			APIToken:      os.Getenv("JIRA_API_TOKEN"),
			ProjectKey:    os.Getenv("JIRA_PROJECT_KEY"),
			IssueType:     getenv("JIRA_ISSUE_TYPE", "Task"),
			HTTPTimeout:   getDuration("JIRA_HTTP_TIMEOUT", 10*time.Second),
			RetryInterval: getDuration("JIRA_RETRY_INTERVAL", 5*time.Minute),
			MaxRetries:    getInt("JIRA_MAX_RETRIES", 10),
		},
		News: NewsConfig{
			FeedURL:  getenv("NEWS_FEED_URL", "https://feeds.feedburner.com/TheHackersNews"),
			Limit:    getInt("NEWS_LIMIT", 5),
			CacheTTL: getDuration("NEWS_CACHE_TTL", 15*time.Minute),
		},
	}

	if cfg.Jira.Enabled {
		if cfg.Jira.BaseURL == "" || cfg.Jira.Email == "" || cfg.Jira.APIToken == "" || cfg.Jira.ProjectKey == "" {
			return cfg, fmt.Errorf("JIRA habilitado pero faltan JIRA_BASE_URL, JIRA_EMAIL, JIRA_API_TOKEN o JIRA_PROJECT_KEY")
		}
	}

	return cfg, nil
}

func buildDSN() (string, error) {
	user := os.Getenv("DB_USER")
	pass := os.Getenv("DB_PASSWORD")
	host := getenv("DB_HOST", "localhost")
	port := getenv("DB_PORT", "5432")
	name := os.Getenv("DB_NAME")

	if user == "" || name == "" {
		return "", fmt.Errorf("DB_USER y DB_NAME son obligatorios")
	}

	u := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(user, pass),
		Host:   fmt.Sprintf("%s:%s", host, port),
		Path:   name,
	}
	q := u.Query()
	q.Set("sslmode", getenv("DB_SSLMODE", "disable"))
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func getenv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return def
}

func getInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}

func getBool(key string, def bool) bool {
	if v := os.Getenv(key); v != "" {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return def
}

// parseCSV separa una cadena por comas y descarta entradas vacías.
// Útil para variables de entorno con varios valores (ej: ALLOWED_ORIGINS).
func parseCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}
