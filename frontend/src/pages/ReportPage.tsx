import { useState, type FormEvent } from 'react'
import { ApiError } from '../api/client'
import { createIncident } from '../api/incidents'
import type { IncidentResponse } from '../api/types'

interface FormState {
  title: string
  description: string
  author: string
}

const initial: FormState = { title: '', description: '', author: '' }

export default function ReportPage() {
  const [form, setForm] = useState<FormState>(initial)
  const [submitting, setSubmitting] = useState(false)
  const [success, setSuccess] = useState<IncidentResponse | null>(null)
  const [error, setError] = useState<string | null>(null)

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((s) => ({ ...s, [key]: value }))
  }

  async function onSubmit(e: FormEvent<HTMLFormElement>) {
    e.preventDefault()
    setError(null)
    setSuccess(null)
    setSubmitting(true)
    try {
      const res = await createIncident({
        title: form.title.trim(),
        description: form.description.trim(),
        author: form.author.trim() || undefined,
      })
      setSuccess(res)
      setForm(initial)
    } catch (e) {
      if (e instanceof ApiError) setError(e.body)
      else setError('Error inesperado al enviar el reporte')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <section>
      <h1>Reportar incidente de seguridad</h1>

      {success && (
        <div className="alert alert-success">
          Incidente <code>{success.id}</code> registrado correctamente
          {success.jira_sync && success.jira_issue_key
            ? ` y sincronizado con JIRA (${success.jira_issue_key}).`
            : '. Se sincronizará con JIRA en el próximo intento del worker.'}
        </div>
      )}

      {error && <div className="alert alert-error">{error}</div>}

      <form onSubmit={onSubmit} className="card">
        <div className="field">
          <label className="label" htmlFor="title">
            Título <span style={{ color: 'var(--error)' }}>*</span>
          </label>
          <input
            id="title"
            className="input"
            type="text"
            value={form.title}
            onChange={(e) => update('title', e.target.value)}
            required
            minLength={3}
            maxLength={255}
            placeholder="Ej: Phishing reportado al correo corporativo"
          />
          <div className="field-hint">
            3 a 255 caracteres. <span className="char-count">{form.title.length}/255</span>
          </div>
        </div>

        <div className="field">
          <label className="label" htmlFor="description">
            Descripción <span style={{ color: 'var(--error)' }}>*</span>
          </label>
          <textarea
            id="description"
            className="textarea"
            value={form.description}
            onChange={(e) => update('description', e.target.value)}
            required
            minLength={5}
            maxLength={10000}
            placeholder="Detalle del incidente, fechas, indicadores observables…"
          />
          <div className="field-hint">
            Mínimo 5 caracteres. <span className="char-count">{form.description.length}/10000</span>
          </div>
        </div>

        <div className="field">
          <label className="label" htmlFor="author">
            Autor <span className="card-meta">(opcional)</span>
          </label>
          <input
            id="author"
            className="input"
            type="text"
            value={form.author}
            onChange={(e) => update('author', e.target.value)}
            maxLength={100}
            placeholder="Tu nombre o usuario corporativo"
          />
          <div className="field-hint">
            <span className="char-count">{form.author.length}/100</span>
          </div>
        </div>

        <button type="submit" className="button" disabled={submitting}>
          {submitting ? 'Enviando…' : 'Enviar reporte'}
        </button>
      </form>
    </section>
  )
}
