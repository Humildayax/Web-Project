package jira

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"sync"

	"security-portal/internal/config"
	"security-portal/internal/models"
)

// Client abstrae la integración con JIRA (u otro tracker).
type Client interface {
	CreateIssue(ctx context.Context, incident *models.Incident) (IssueRef, error)
	// AttachToIssue sube un archivo binario al ticket. JIRA Cloud requiere
	// multipart/form-data con field "file" + header X-Atlassian-Token: no-check.
	AttachToIssue(ctx context.Context, issueKey, filename, mimeType string, data []byte) error
}

// IssueRef es lo que devuelve el tracker tras crear el ticket.
type IssueRef struct {
	Key string
	URL string
}

// -------- Implementación HTTP (JIRA Cloud REST API v3) --------

type httpClient struct {
	cfg  config.JiraConfig
	http *http.Client
}

func NewHTTPClient(cfg config.JiraConfig) Client {
	return &httpClient{
		cfg:  cfg,
		http: &http.Client{Timeout: cfg.HTTPTimeout},
	}
}

// Atlassian Document Format (ADF) mínimo: párrafos con texto plano.
type adfDoc struct {
	Type    string     `json:"type"`
	Version int        `json:"version"`
	Content []adfBlock `json:"content"`
}

type adfBlock struct {
	Type    string      `json:"type"`
	Content []adfInline `json:"content"`
}

type adfInline struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type createIssueResp struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Self string `json:"self"`
}

// adfFromIncident arma el body ADF del ticket: dos párrafos, autor primero
// y descripción del usuario después. Mantenerlos separados es más correcto
// que meter "\n\n" en un mismo párrafo (ADF no interpreta saltos así).
func adfFromIncident(incident *models.Incident) adfDoc {
	return adfDoc{
		Type:    "doc",
		Version: 1,
		Content: []adfBlock{
			{
				Type: "paragraph",
				Content: []adfInline{{
					Type: "text",
					Text: fmt.Sprintf("Reportado por: %s", incident.Author),
				}},
			},
			{
				Type: "paragraph",
				Content: []adfInline{{
					Type: "text",
					Text: incident.Description,
				}},
			},
		},
	}
}

// requiredFields son los que mandamos en el POST inicial: project,
// summary, issuetype, description. Estos suelen estar siempre presentes
// en el "Create Issue Screen" de cualquier proyecto.
func requiredFields(cfg config.JiraConfig, incident *models.Incident) map[string]any {
	return map[string]any{
		"project":     map[string]string{"key": cfg.ProjectKey},
		"summary":     truncate(incident.Title, 255),
		"issuetype":   map[string]string{"name": cfg.IssueType},
		"description": adfFromIncident(incident),
	}
}

// optionalFields son los que mandamos en un PUT post-creación.
// JIRA Cloud descarta silenciosamente los campos que no están en el
// "Create Issue Screen" del proyecto destino y devuelve 201 OK como si
// todo hubiera estado bien. La pantalla de edición suele ser más
// permisiva, así que aplicamos los opcionales con un PUT separado.
//
// Soporta dos modelos de "padre":
//   - parent.id  → JIRA Cloud nuevo (next-gen / company-managed con
//     jerarquía de epics moderna).
//   - customfield_10014 (Epic Link) → JIRA Software Classic. El valor
//     debe ser la KEY del epic (ej. "CYBER-1"), no el id numérico.
//
// Si la config trae ambas, mandamos las dos; JIRA usa la que aplique.
func optionalFields(cfg config.JiraConfig, incident *models.Incident) map[string]any {
	fields := map[string]any{}
	if cfg.AssigneeAccountID != "" {
		fields["assignee"] = map[string]string{"accountId": cfg.AssigneeAccountID}
	}
	if cfg.ReporterAccountID != "" {
		fields["reporter"] = map[string]string{"accountId": cfg.ReporterAccountID}
	}
	if cfg.PriorityName != "" {
		fields["priority"] = map[string]string{"name": cfg.PriorityName}
	}
	if cfg.ParentID != "" {
		fields["parent"] = map[string]string{"id": cfg.ParentID}
	}
	if cfg.EpicLinkFieldID != "" && cfg.EpicLinkValue != "" {
		fields[cfg.EpicLinkFieldID] = cfg.EpicLinkValue
	}
	if cfg.StartDateFieldID != "" {
		// "Start date" en JIRA Cloud es de tipo `date`, formato ISO YYYY-MM-DD.
		// Si fuera datetime habría que usar time.RFC3339.
		fields[cfg.StartDateFieldID] = incident.CreatedAt.UTC().Format("2006-01-02")
	}
	return fields
}

func (c *httpClient) CreateIssue(ctx context.Context, incident *models.Incident) (IssueRef, error) {
	created, err := c.postIssue(ctx, requiredFields(c.cfg, incident))
	if err != nil {
		return IssueRef{}, err
	}

	// Aplicamos cada campo opcional con un PUT independiente, en paralelo.
	// Si JIRA rechaza un campo (no está en el edit screen, valor inválido,
	// etc.) afecta SOLO a ese campo: los demás se aplican igual. Mandar
	// todo en un único PUT genera rollback total si uno falla.
	if extras := optionalFields(c.cfg, incident); len(extras) > 0 {
		c.applyOptionalFields(ctx, created.Key, extras)
	}

	return IssueRef{
		Key: created.Key,
		URL: fmt.Sprintf("%s/browse/%s", c.cfg.BaseURL, created.Key),
	}, nil
}

// applyOptionalFields aplica cada campo con un PUT separado en paralelo.
// Loggea éxito/fallo por campo para diagnóstico granular. Nunca retorna
// error: el ticket ya existe y la falla en un campo opcional no debe
// invalidar el sync.
func (c *httpClient) applyOptionalFields(ctx context.Context, key string, fields map[string]any) {
	type result struct {
		field string
		err   error
	}
	results := make([]result, 0, len(fields))
	var mu sync.Mutex
	var wg sync.WaitGroup

	for k, v := range fields {
		wg.Add(1)
		go func(field string, value any) {
			defer wg.Done()
			err := c.updateIssue(ctx, key, map[string]any{field: value})
			mu.Lock()
			results = append(results, result{field: field, err: err})
			mu.Unlock()
		}(k, v)
	}
	wg.Wait()

	applied := make([]string, 0, len(results))
	for _, r := range results {
		if r.err != nil {
			slog.Warn("jira: campo opcional rechazado",
				"key", key, "field", r.field, "err", r.err)
			continue
		}
		applied = append(applied, r.field)
	}
	slog.Info("jira: campos opcionales procesados",
		"key", key, "applied", applied, "failed_count", len(fields)-len(applied))
}

// postIssue hace el POST inicial al endpoint /rest/api/3/issue y devuelve
// el id/key del ticket creado.
func (c *httpClient) postIssue(ctx context.Context, fields map[string]any) (createIssueResp, error) {
	body, err := json.Marshal(map[string]any{"fields": fields})
	if err != nil {
		return createIssueResp{}, fmt.Errorf("marshal: %w", err)
	}

	endpoint := fmt.Sprintf("%s/rest/api/3/issue", c.cfg.BaseURL)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return createIssueResp{}, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.cfg.Email, c.cfg.APIToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return createIssueResp{}, fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return createIssueResp{}, fmt.Errorf("jira POST issue status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed createIssueResp
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return createIssueResp{}, fmt.Errorf("decode: %w (body=%s)", err, string(respBody))
	}
	slog.Info("jira: ticket creado", "key", parsed.Key, "id", parsed.ID)
	return parsed, nil
}

// AttachToIssue sube un archivo al ticket vía multipart/form-data.
//
// JIRA Cloud requiere el header X-Atlassian-Token: no-check (deshabilita un
// check de XSRF que no aplica a auth por API token). El field se llama
// literalmente "file" — la API admite mandar varios "file" en una sola
// llamada, pero subimos uno por uno desde el worker para que el tracking
// de éxitos parciales sea más simple.
func (c *httpClient) AttachToIssue(ctx context.Context, issueKey, filename, mimeType string, data []byte) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	// Construimos el part a mano para poder fijar el Content-Type del
	// archivo. CreateFormFile no lo permite (siempre setea octet-stream).
	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(
		`form-data; name="file"; filename="%s"`,
		quoteEscape(filename),
	))
	if mimeType != "" {
		h.Set("Content-Type", mimeType)
	}
	part, err := writer.CreatePart(h)
	if err != nil {
		return fmt.Errorf("multipart create part: %w", err)
	}
	if _, err := part.Write(data); err != nil {
		return fmt.Errorf("multipart write: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("multipart close: %w", err)
	}

	endpoint := fmt.Sprintf("%s/rest/api/3/issue/%s/attachments", c.cfg.BaseURL, issueKey)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("X-Atlassian-Token", "no-check")
	req.SetBasicAuth(c.cfg.Email, c.cfg.APIToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// quoteEscape escapa los caracteres que romperían el header
// `filename="..."`. RFC 7578 permite \" y \\ — el resto pasa tal cual.
// El filename ya viene sanitizado del lado de la app (DTO.SanitizeFilename),
// pero el escape es defensa en profundidad.
func quoteEscape(s string) string {
	r := strings.NewReplacer(`\`, `\\`, `"`, `\"`)
	return r.Replace(s)
}

// updateIssue hace PUT a /rest/api/3/issue/{keyOrID} con los fields dados.
// Pensado para llamarse con UN campo por vez desde applyOptionalFields,
// así un campo rechazado no rollbackea a los demás.
//
// JIRA responde 204 No Content cuando OK. Si falla por permisos o
// validación, devuelve 4xx con detalle en el body.
func (c *httpClient) updateIssue(ctx context.Context, keyOrID string, fields map[string]any) error {
	body, err := json.Marshal(map[string]any{"fields": fields})
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}

	endpoint := fmt.Sprintf("%s/rest/api/3/issue/%s", c.cfg.BaseURL, keyOrID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.SetBasicAuth(c.cfg.Email, c.cfg.APIToken)

	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("http: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		respBody, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("status %d: %s", resp.StatusCode, string(respBody))
	}
	return nil
}

// truncate corta un string a n runes (no bytes). Importante para UTF-8:
// cortar por bytes puede partir un code-point a la mitad y romper el JSON.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) > n {
		return string(r[:n])
	}
	return s
}

// -------- No-op (modo desarrollo sin credenciales reales) --------

type noopClient struct{}

func NewNoopClient() Client { return &noopClient{} }

func (noopClient) CreateIssue(_ context.Context, incident *models.Incident) (IssueRef, error) {
	slog.Info("jira noop: marcando como sincronizado sin llamar a la API", "incident_id", incident.ID)
	return IssueRef{Key: "NOOP-LOCAL"}, nil
}

func (noopClient) AttachToIssue(_ context.Context, issueKey, filename, _ string, data []byte) error {
	slog.Info("jira noop: attach simulado",
		"issue_key", issueKey, "filename", filename, "size", len(data))
	return nil
}
