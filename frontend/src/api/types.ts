// =====================================================================
// Tipos espejo de los DTOs del backend (Go).
// Mantener este archivo en sincronía con backend/internal/dto/incident.go
// y backend/internal/models/news.go.
//
// Este es el ÚNICO punto de acoplamiento entre front y back.
// =====================================================================

// ---- Incidentes ----

// POST /api/incidents
// Espejo de dto.CreateIncidentRequest
export interface CreateIncidentRequest {
  title: string
  description: string
  author?: string
}

// Respuesta 201 de POST /api/incidents
// Espejo de dto.IncidentResponse
export interface IncidentResponse {
  id: string                   // uuid serializado como string
  title: string
  description: string
  author: string
  created_at: string           // ISO timestamp
  jira_sync: boolean
  jira_issue_key?: string
}

// ---- Noticias ----

// GET /api/news
// Espejo de models.NewsItem
export interface NewsItem {
  title: string
  link: string
  published_at: string
  source: string
}

// ---- Errores ----

// Forma estándar que devuelve el backend ante 4xx/5xx.
export interface ErrorResponse {
  error: string
}
