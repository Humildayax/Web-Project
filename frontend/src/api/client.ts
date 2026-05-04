import type { ErrorResponse } from './types'

const BASE_URL: string = import.meta.env.VITE_API_BASE_URL ?? 'http://localhost:8080'

/**
 * ApiError representa cualquier respuesta HTTP no-2xx del backend.
 * `status` es el código HTTP, `body` es el mensaje (preferentemente
 * el campo `error` del JSON estándar; si no, el texto crudo).
 */
export class ApiError extends Error {
  constructor(
    public readonly status: number,
    public readonly body: string,
  ) {
    super(`API ${status}: ${body}`)
    this.name = 'ApiError'
  }
}

/**
 * request es el wrapper único de fetch. Centraliza:
 *   - baseURL
 *   - headers JSON por defecto (no se aplican a FormData)
 *   - manejo de errores tipado (ApiError)
 *
 * Si el body es FormData, NO seteamos Content-Type: el browser lo arma con
 * el boundary correcto. Forzar application/json en multipart rompe el parser.
 *
 * Ningún módulo fuera de `api/` debería usar fetch directo.
 */
export async function request<T>(path: string, init: RequestInit = {}): Promise<T> {
  const isFormData = init.body instanceof FormData

  const headers: Record<string, string> = {
    Accept: 'application/json',
    ...((init.headers as Record<string, string>) ?? {}),
  }
  if (!isFormData && !headers['Content-Type']) {
    headers['Content-Type'] = 'application/json'
  }

  const res = await fetch(`${BASE_URL}${path}`, { ...init, headers })

  if (!res.ok) {
    const raw = await res.text()
    let message = raw
    try {
      const parsed = JSON.parse(raw) as Partial<ErrorResponse>
      if (parsed.error) message = parsed.error
    } catch {
      // body no era JSON, dejamos el texto crudo
    }
    throw new ApiError(res.status, message)
  }

  if (res.status === 204) return undefined as T
  return (await res.json()) as T
}
