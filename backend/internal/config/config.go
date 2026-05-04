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
	HTTP       HTTPConfig
	DB         DBConfig
	Jira       JiraConfig
	News       NewsConfig
	Retention  RetentionConfig
	Storage    StorageConfig
	Attachment AttachmentConfig
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

	// Rate-limit del endpoint POST /api/incidents.
	// Se aplica por IP del cliente (ver handlers.clientIP) y se cuenta
	// en una ventana deslizante de IncidentRateWindow.
	IncidentRateLimit  int
	IncidentRateWindow time.Duration
}

type DBConfig struct {
	DSN               string
	MaxConns          int32
	MinConns          int32
	MaxConnLifetime   time.Duration
	MaxConnIdleTime   time.Duration
	HealthCheckPeriod time.Duration

	// OpTimeout es el deadline por operación de repositorio (Create, Mark*,
	// Claim*…). No aplica a operaciones largas como PurgeOldMetadata, que
	// fija su propio timeout interno.
	OpTimeout time.Duration

	// MigrateOnStart controla si el binario aplica migraciones al arrancar.
	// Default: true. Apagar en entornos donde las migraciones se corren
	// por separado (CI, herramienta externa, operador).
	MigrateOnStart bool
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
	// BaseBackoff es el backoff inicial entre reintentos. El backoff real es
	// BaseBackoff * 2^sync_retries: el primer fallo espera BaseBackoff, el
	// segundo 2*BaseBackoff, etc. Evita que el worker martille a JIRA si
	// está caído.
	BaseBackoff time.Duration

	// Campos opcionales del ticket. Si una variable está vacía, el campo se
	// OMITE del payload (no se manda como null) y JIRA aplica su default.
	// Esto permite ir activando campos uno por uno mientras se ajusta la
	// instancia destino.
	AssigneeAccountID string // accountId del usuario; obtenerlo de /rest/api/3/myself
	ReporterAccountID string // accountId; típicamente el mismo que assignee
	PriorityName      string // nombre tal como aparece en JIRA: "High", "Medium", etc.
	ParentID          string // id numérico del issue padre (jerarquía / epic)
	// StartDateFieldID es el id del custom field "Start date" en TU instancia
	// (algo tipo "customfield_10015"). Si está seteado, se mapea con la
	// fecha de creación del incidente (formato YYYY-MM-DD).
	StartDateFieldID string

	// Epic Link para JIRA Software Classic. En proyectos modernos el padre
	// se setea con `parent.id` (ParentID arriba); en Classic había que usar
	// un custom field específico (típicamente customfield_10014) y como
	// valor la KEY del epic ("CYBER-1"), no el id numérico.
	//
	// Si los dos están vacíos, este mapeo se omite. Si parent ya funciona
	// en tu instancia, no necesitás setear estos.
	EpicLinkFieldID string
	EpicLinkValue   string

	// AttachmentBatchSize es cuántos adjuntos sube el worker a JIRA por
	// tick. Mantiene la latencia del worker acotada cuando hay backlog.
	AttachmentBatchSize int
}

type NewsConfig struct {
	FeedURLs []string
	// LimitPerSource es la cantidad de items que se pide a CADA feed.
	// Con 4 feeds default y LimitPerSource=3 la página termina con ~12
	// noticias agrupadas por fuente.
	LimitPerSource int
	CacheTTL       time.Duration
}

// RetentionConfig controla la purga periódica de metadata (PII) de los
// incidentes. Si IncidentMetadataMaxAge <= 0 el worker no se inicia.
type RetentionConfig struct {
	IncidentMetadataMaxAge time.Duration
	Interval               time.Duration
}

// StorageConfig controla dónde se guardan los binarios de los adjuntos.
// Por ahora solo tipo "local" (filesystem). Migrable a S3/MinIO sumando
// otra implementación de storage.Storage sin tocar el resto del código.
type StorageConfig struct {
	BasePath string
}

// AttachmentConfig define las cotas que el handler aplica antes de
// procesar un upload. Triple defensa contra abuso:
//  - MaxFiles: cuántos archivos por reporte.
//  - MaxFileBytes: tamaño máximo por archivo (post lectura).
//  - MaxImageDim: ancho/alto máximo en pixels (anti compression bomb).
type AttachmentConfig struct {
	MaxFiles       int
	MaxFileBytes   int64
	MaxImageDim    int
	MaxMemoryParse int64 // RAM antes de spillar a temp en multipart parse
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
			// Default subido a 30 MiB para soportar reportes con hasta 5
			// imágenes de 5 MB. nginx hace el primer corte (client_max_body_size).
			MaxBodyBytes:      int64(getInt("HTTP_MAX_BODY_BYTES", 30<<20)),
			AllowedOrigins:    parseAllowedOrigins(),

			IncidentRateLimit:  getInt("INCIDENT_RATE_LIMIT", 10),
			IncidentRateWindow: getDuration("INCIDENT_RATE_WINDOW", time.Minute),
		},
		DB: DBConfig{
			DSN:               dsn,
			MaxConns:          int32(getInt("DB_MAX_CONNS", 10)),
			MinConns:          int32(getInt("DB_MIN_CONNS", 2)),
			MaxConnLifetime:   getDuration("DB_MAX_CONN_LIFETIME", time.Hour),
			MaxConnIdleTime:   getDuration("DB_MAX_CONN_IDLE", 30*time.Minute),
			HealthCheckPeriod: getDuration("DB_HEALTHCHECK_PERIOD", time.Minute),
			OpTimeout:         getDuration("DB_OP_TIMEOUT", 5*time.Second),
			MigrateOnStart:    getBool("MIGRATE_ON_START", true),
		},
		Jira: JiraConfig{
			Enabled:           getBool("JIRA_ENABLED", false),
			BaseURL:           os.Getenv("JIRA_BASE_URL"),
			Email:             os.Getenv("JIRA_EMAIL"),
			APIToken:          os.Getenv("JIRA_API_TOKEN"),
			ProjectKey:        os.Getenv("JIRA_PROJECT_KEY"),
			IssueType:         getenv("JIRA_ISSUE_TYPE", "Task"),
			HTTPTimeout:       getDuration("JIRA_HTTP_TIMEOUT", 10*time.Second),
			RetryInterval:     getDuration("JIRA_RETRY_INTERVAL", 5*time.Minute),
			MaxRetries:        getInt("JIRA_MAX_RETRIES", 10),
			BaseBackoff:       getDuration("JIRA_RETRY_BASE_BACKOFF", 30*time.Second),
			AssigneeAccountID: os.Getenv("JIRA_ASSIGNEE_ACCOUNT_ID"),
			ReporterAccountID: os.Getenv("JIRA_REPORTER_ACCOUNT_ID"),
			PriorityName:      os.Getenv("JIRA_PRIORITY_NAME"),
			ParentID:          os.Getenv("JIRA_PARENT_ID"),
			StartDateFieldID:  os.Getenv("JIRA_START_DATE_FIELD_ID"),
			EpicLinkFieldID:   os.Getenv("JIRA_EPIC_LINK_FIELD_ID"),
			EpicLinkValue:     os.Getenv("JIRA_EPIC_LINK_VALUE"),

			AttachmentBatchSize: getInt("JIRA_ATTACHMENT_BATCH_SIZE", 20),
		},
		News: NewsConfig{
			FeedURLs:       parseFeedURLs(),
			LimitPerSource: getInt("NEWS_LIMIT_PER_SOURCE", 3),
			CacheTTL:       getDuration("NEWS_CACHE_TTL", 15*time.Minute),
		},
		Retention: RetentionConfig{
			IncidentMetadataMaxAge: getDuration("INCIDENT_METADATA_MAX_AGE", 90*24*time.Hour),
			Interval:               getDuration("RETENTION_INTERVAL", 24*time.Hour),
		},
		Storage: StorageConfig{
			BasePath: getenv("STORAGE_BASE_PATH", "/data/incidents"),
		},
		Attachment: AttachmentConfig{
			MaxFiles:       getInt("ATTACHMENT_MAX_FILES", 5),
			MaxFileBytes:   int64(getInt("ATTACHMENT_MAX_FILE_BYTES", 5<<20)),  // 5 MB
			MaxImageDim:    getInt("ATTACHMENT_MAX_IMAGE_DIM", 4096),
			MaxMemoryParse: int64(getInt("ATTACHMENT_PARSE_MEMORY", 10<<20)),   // 10 MB
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

// parseFeedURLs lee NEWS_FEED_URLS (CSV) o, si no está, devuelve un set
// curado de 4 fuentes:
//   - The Hacker News (EN, internacional, técnico)
//   - WeLiveSecurity / ESET (ES, técnico, calidad alta)
//   - cybersecuritynews.es (ES, actualidad)
//   - impactotic.co (Colombia, contexto local)
//
// Si la variable existe pero queda vacía tras parsear, se usa también el
// default — es más seguro tener noticias que un endpoint roto.
func parseFeedURLs() []string {
	defaults := []string{
		"https://feeds.feedburner.com/TheHackersNews",
		"https://feeds.feedburner.com/welivesecurity",
		"https://cybersecuritynews.es/feed/",
		"https://impactotic.co/ciber-seguridad/feed/",
	}
	v, ok := os.LookupEnv("NEWS_FEED_URLS")
	if !ok {
		return defaults
	}
	urls := parseCSV(v)
	if len(urls) == 0 {
		return defaults
	}
	return urls
}

// parseAllowedOrigins distingue tres casos para ALLOWED_ORIGINS:
//   - unset:        devuelve el default de dev (http://localhost:5173).
//   - seteado y no vacío: parsea la CSV.
//   - seteado y vacío:    devuelve []string{} (CORS deshabilitado en runtime).
//
// Esto es deliberado: en prod el front y el back viven detrás del mismo
// reverse proxy, así que CORS no aplica. Setear ALLOWED_ORIGINS="" desactiva
// el middleware en lugar de quedar con un default permisivo.
func parseAllowedOrigins() []string {
	v, ok := os.LookupEnv("ALLOWED_ORIGINS")
	if !ok {
		return []string{"http://localhost:5173"}
	}
	return parseCSV(v)
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
