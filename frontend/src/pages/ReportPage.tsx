import { useEffect, useState, type ChangeEvent, type FormEvent } from 'react'
import { ApiError } from '../api/client'
import { createIncident } from '../api/incidents'
import type { IncidentResponse } from '../api/types'

interface FormState {
  title: string
  description: string
  author: string
}

const initial: FormState = { title: '', description: '', author: '' }

// Estos límites espejan los del backend (config.AttachmentConfig). El
// cliente los aplica solo como UX; el backend los re-valida y es la
// autoridad final.
const MAX_FILES = 5
const MAX_FILE_BYTES = 5 * 1024 * 1024 // 5 MB
const ACCEPTED_TYPES = new Set(['image/jpeg', 'image/png', 'image/webp'])
const ACCEPTED_HINT = 'JPG, PNG o WEBP'

interface PendingFile {
  id: string          // generado en cliente para keys de React
  file: File
  previewUrl: string  // object URL para mostrar el thumbnail
}

function humanBytes(n: number): string {
  if (n < 1024) return `${n} B`
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(1)} KB`
  return `${(n / (1024 * 1024)).toFixed(1)} MB`
}

export default function ReportPage() {
  const [form, setForm] = useState<FormState>(initial)
  const [files, setFiles] = useState<PendingFile[]>([])
  const [submitting, setSubmitting] = useState(false)
  const [success, setSuccess] = useState<IncidentResponse | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [fileError, setFileError] = useState<string | null>(null)

  // Cleanup de object URLs al desmontar o reemplazar archivos. Sin esto
  // el browser acumula memoria por cada preview generada.
  useEffect(() => {
    return () => {
      for (const f of files) URL.revokeObjectURL(f.previewUrl)
    }
  }, [files])

  function update<K extends keyof FormState>(key: K, value: FormState[K]) {
    setForm((s) => ({ ...s, [key]: value }))
  }

  function onPickFiles(e: ChangeEvent<HTMLInputElement>) {
    setFileError(null)
    const picked = Array.from(e.target.files ?? [])
    // El input se limpia para que el usuario pueda re-elegir el mismo
    // archivo si lo quitó por error.
    e.target.value = ''
    if (picked.length === 0) return

    const remaining = MAX_FILES - files.length
    if (picked.length > remaining) {
      setFileError(`Podés adjuntar hasta ${MAX_FILES} archivos (te quedan ${remaining}).`)
      return
    }

    const accepted: PendingFile[] = []
    for (const f of picked) {
      if (!ACCEPTED_TYPES.has(f.type)) {
        setFileError(`"${f.name}" no es una imagen permitida (${ACCEPTED_HINT}).`)
        // revocar las URLs ya creadas en este batch
        for (const a of accepted) URL.revokeObjectURL(a.previewUrl)
        return
      }
      if (f.size > MAX_FILE_BYTES) {
        setFileError(`"${f.name}" supera el máximo de 5 MB.`)
        for (const a of accepted) URL.revokeObjectURL(a.previewUrl)
        return
      }
      accepted.push({
        id: `${f.name}-${f.size}-${f.lastModified}-${Math.random().toString(36).slice(2)}`,
        file: f,
        previewUrl: URL.createObjectURL(f),
      })
    }
    setFiles((cur) => [...cur, ...accepted])
  }

  function removeFile(id: string) {
    setFiles((cur) => {
      const target = cur.find((f) => f.id === id)
      if (target) URL.revokeObjectURL(target.previewUrl)
      return cur.filter((f) => f.id !== id)
    })
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
        attachments: files.length > 0 ? files.map((f) => f.file) : undefined,
      })
      setSuccess(res)
      // Limpiar form + previews
      setForm(initial)
      for (const f of files) URL.revokeObjectURL(f.previewUrl)
      setFiles([])
    } catch (e) {
      if (e instanceof ApiError) setError(e.body)
      else setError('Error inesperado al enviar el reporte')
    } finally {
      setSubmitting(false)
    }
  }

  return (
    <div className="container py-4 py-md-5">
      <div className="row justify-content-center">
        <div className="col-lg-8">
          <h1 className="fw-bold mb-4">Reportar incidente de seguridad</h1>

          {success && (
            <div className="alert alert-success" role="alert">
              Incidente <code>{success.id}</code> registrado correctamente
              {success.jira_sync && success.jira_issue_key
                ? ` y sincronizado con JIRA (${success.jira_issue_key}).`
                : '. Se sincronizará con JIRA en el próximo intento del worker.'}
              {success.attachments && success.attachments.length > 0 && (
                <span> Se adjuntaron {success.attachments.length} archivo(s).</span>
              )}
            </div>
          )}

          {error && (
            <div className="alert alert-danger" role="alert">
              {error}
            </div>
          )}

          <form onSubmit={onSubmit} className="card">
            <div className="card-body p-4">
              <div className="mb-3">
                <label className="form-label fw-semibold" htmlFor="title">
                  Título <span className="text-danger">*</span>
                </label>
                <input
                  id="title"
                  className="form-control"
                  type="text"
                  value={form.title}
                  onChange={(e) => update('title', e.target.value)}
                  required
                  minLength={3}
                  maxLength={255}
                  placeholder="Ej: Phishing reportado al correo corporativo"
                />
                <div className="form-text">
                  3 a 255 caracteres. <span className="char-count">{form.title.length}/255</span>
                </div>
              </div>

              <div className="mb-3">
                <label className="form-label fw-semibold" htmlFor="description">
                  Descripción <span className="text-danger">*</span>
                </label>
                <textarea
                  id="description"
                  className="form-control"
                  rows={6}
                  value={form.description}
                  onChange={(e) => update('description', e.target.value)}
                  required
                  minLength={5}
                  maxLength={10000}
                  placeholder="Detalle del incidente, fechas, indicadores observables…"
                />
                <div className="form-text">
                  Mínimo 5 caracteres. <span className="char-count">{form.description.length}/10000</span>
                </div>
              </div>

              <div className="mb-3">
                <label className="form-label fw-semibold" htmlFor="author">
                  Autor <span className="text-muted small">(opcional)</span>
                </label>
                <input
                  id="author"
                  className="form-control"
                  type="text"
                  value={form.author}
                  onChange={(e) => update('author', e.target.value)}
                  maxLength={100}
                  placeholder="Tu nombre o usuario corporativo"
                />
                <div className="form-text">
                  <span className="char-count">{form.author.length}/100</span>
                </div>
              </div>

              <div className="mb-4">
                <label className="form-label fw-semibold" htmlFor="attachments">
                  Imágenes <span className="text-muted small">(opcional, hasta {MAX_FILES})</span>
                </label>
                <input
                  id="attachments"
                  className="form-control"
                  type="file"
                  accept="image/jpeg,image/png,image/webp"
                  multiple
                  onChange={onPickFiles}
                  disabled={files.length >= MAX_FILES}
                />
                <div className="form-text">
                  {ACCEPTED_HINT} · máx 5 MB por archivo · {files.length}/{MAX_FILES} adjuntos.
                </div>
                {fileError && (
                  <div className="alert alert-warning mt-2 mb-0 py-2 small" role="alert">
                    {fileError}
                  </div>
                )}

                {files.length > 0 && (
                  <div className="row g-2 mt-2">
                    {files.map((f) => (
                      <div key={f.id} className="col-6 col-md-4 col-lg-3">
                        <div className="position-relative border rounded overflow-hidden bg-light">
                          <img
                            src={f.previewUrl}
                            alt={f.file.name}
                            style={{
                              width: '100%',
                              height: '100px',
                              objectFit: 'cover',
                              display: 'block',
                            }}
                          />
                          <button
                            type="button"
                            className="btn btn-sm btn-dark position-absolute top-0 end-0 m-1 py-0 px-1"
                            onClick={() => removeFile(f.id)}
                            aria-label={`Quitar ${f.file.name}`}
                          >
                            ✕
                          </button>
                          <div className="px-2 py-1 small text-truncate" title={f.file.name}>
                            {f.file.name}
                          </div>
                          <div className="px-2 pb-1 small text-muted">
                            {humanBytes(f.file.size)}
                          </div>
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>

              <button type="submit" className="btn btn-primary" disabled={submitting}>
                {submitting && (
                  <span
                    className="spinner-border spinner-border-sm me-2"
                    role="status"
                    aria-hidden="true"
                  />
                )}
                {submitting ? 'Enviando…' : 'Enviar reporte'}
              </button>
            </div>
          </form>
        </div>
      </div>
    </div>
  )
}
