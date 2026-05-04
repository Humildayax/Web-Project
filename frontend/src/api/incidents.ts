import { request } from './client'
import type { CreateIncidentRequest, IncidentResponse } from './types'

export function createIncident(body: CreateIncidentRequest): Promise<IncidentResponse> {
  // multipart/form-data: el browser arma el boundary cuando le pasamos
  // FormData como body. No setear Content-Type a mano.
  const fd = new FormData()
  fd.append('title', body.title)
  fd.append('description', body.description)
  if (body.author) fd.append('author', body.author)
  if (body.attachments) {
    for (const file of body.attachments) {
      fd.append('attachments', file, file.name)
    }
  }
  return request<IncidentResponse>('/api/incidents', {
    method: 'POST',
    body: fd,
  })
}
