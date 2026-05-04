import { useEffect, useState } from 'react'

// Cómo agregar una política nueva (sin tocar este componente):
//   1. Copiar el PDF a `frontend/public/policies/<nombre>.pdf`.
//   2. Sumar una entrada al array de `frontend/public/policies/index.json`
//      con id único, título, descripción, filename y updated_at.
// nginx sirve los archivos estáticos del directorio public/ tal cual; el
// rebuild del frontend reempaca el PDF al bundle /usr/share/nginx/html.
//
// El visor inline es un iframe al PDF (mismo origen). En navegadores que no
// soportan PDF nativo (algunos móviles) el botón "Abrir en pestaña nueva"
// fuerza la descarga / visor del SO.

interface Policy {
  id: string
  title: string
  description?: string
  filename: string
  updated_at?: string
}

const INDEX_URL = '/policies/index.json'
const POLICIES_BASE = '/policies/'

function policyHref(p: Policy): string {
  return POLICIES_BASE + p.filename
}

export default function PoliciesPage() {
  const [policies, setPolicies] = useState<Policy[] | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [selectedId, setSelectedId] = useState<string | null>(null)

  useEffect(() => {
    const ac = new AbortController()
    fetch(INDEX_URL, { signal: ac.signal, headers: { Accept: 'application/json' } })
      .then((res) => {
        if (!res.ok) throw new Error(`HTTP ${res.status}`)
        return res.json() as Promise<Policy[]>
      })
      .then((list) => {
        setPolicies(list)
        if (list.length === 1) setSelectedId(list[0].id)
      })
      .catch((e: unknown) => {
        if (e instanceof DOMException && e.name === 'AbortError') return
        setError('No se pudo cargar el índice de políticas.')
      })
    return () => ac.abort()
  }, [])

  const selected = policies?.find((p) => p.id === selectedId) ?? null

  return (
    <div className="container py-4 py-md-5">
      <h1 className="fw-bold mb-3">Políticas de seguridad</h1>
      <p className="text-muted mb-4">
        Seleccioná un documento para abrirlo en el visor. Si el visor no carga,
        usá el botón de abrir en pestaña nueva.
      </p>

      {error && (
        <div className="alert alert-danger" role="alert">
          {error}
        </div>
      )}

      {!policies && !error && (
        <div className="text-center text-muted py-5">
          <div className="spinner-border text-primary mb-2" role="status" aria-hidden="true" />
          <div>Cargando…</div>
        </div>
      )}

      {policies && policies.length === 0 && (
        <div className="alert alert-info">
          Aún no hay políticas publicadas.
        </div>
      )}

      {policies && policies.length > 0 && (
        <div className="row g-3 mb-4">
          {policies.map((p) => {
            const isSelected = p.id === selectedId
            return (
              <div key={p.id} className="col-md-6 col-lg-4">
                <button
                  type="button"
                  onClick={() => setSelectedId(p.id)}
                  className={
                    'card h-100 text-start w-100 border ' +
                    (isSelected ? 'border-primary border-2' : 'border-1')
                  }
                  aria-pressed={isSelected}
                >
                  <div className="card-body">
                    <h2 className="h6 fw-semibold mb-1">{p.title}</h2>
                    {p.description && (
                      <p className="card-text small text-muted mb-2">{p.description}</p>
                    )}
                    <div className="small text-muted">
                      {p.updated_at && <span>Actualizado: {p.updated_at}</span>}
                    </div>
                  </div>
                </button>
              </div>
            )
          })}
        </div>
      )}

      {selected && (
        <section aria-labelledby="viewer-title" className="mt-4">
          <div className="d-flex flex-wrap align-items-center justify-content-between mb-2 gap-2">
            <h2 id="viewer-title" className="h5 fw-bold mb-0">
              {selected.title}
            </h2>
            <a
              href={policyHref(selected)}
              target="_blank"
              rel="noreferrer"
              className="btn btn-outline-primary btn-sm"
            >
              Abrir en pestaña nueva
            </a>
          </div>
          <iframe
            key={selected.id}
            src={policyHref(selected)}
            title={selected.title}
            className="pdf-frame"
          />
        </section>
      )}
    </div>
  )
}
