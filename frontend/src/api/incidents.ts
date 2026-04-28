import { request } from './client'
import type { CreateIncidentRequest, IncidentResponse } from './types'

export function createIncident(body: CreateIncidentRequest): Promise<IncidentResponse> {
  return request<IncidentResponse>('/api/incidents', {
    method: 'POST',
    body: JSON.stringify(body),
  })
}
