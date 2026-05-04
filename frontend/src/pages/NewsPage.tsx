import { useEffect, useMemo, useState } from 'react'
import { ApiError } from '../api/client'
import { getNews } from '../api/news'
import type { NewsItem } from '../api/types'

export default function NewsPage() {
  const [items, setItems] = useState<NewsItem[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    // AbortController evita dos cosas:
    //  1. setState sobre componente desmontado si el usuario navega antes
    //     de que llegue la respuesta.
    //  2. En dev con StrictMode, React monta-desmonta-monta cada componente,
    //     y sin abort el primer fetch sigue corriendo.
    const ac = new AbortController()
    getNews({ signal: ac.signal })
      .then(setItems)
      .catch((e: unknown) => {
        if (e instanceof DOMException && e.name === 'AbortError') return
        if (e instanceof ApiError) setError(e.body)
        else setError('No se pudieron cargar las noticias')
      })
    return () => ac.abort()
  }, [])

  // Agrupar por fuente preservando el orden en que llegan del backend.
  // Map mantiene insertion order en JS, así que el orden visual coincide
  // con el orden de NEWS_FEED_URLS en el backend.
  const groups = useMemo(() => {
    if (!items) return null
    const map = new Map<string, NewsItem[]>()
    for (const item of items) {
      const source = item.source || 'Otras fuentes'
      const list = map.get(source)
      if (list) list.push(item)
      else map.set(source, [item])
    }
    return Array.from(map.entries())
  }, [items])

  return (
    <div className="container py-4 py-md-5">
      <h1 className="fw-bold mb-4">Noticias de seguridad</h1>

      {error && (
        <div className="alert alert-danger" role="alert">
          {error}
        </div>
      )}

      {!items && !error && (
        <div className="text-center text-muted py-5">
          <div className="spinner-border text-primary mb-2" role="status" aria-hidden="true" />
          <div>Cargando…</div>
        </div>
      )}

      {groups && groups.length === 0 && (
        <div className="alert alert-info">No hay noticias por el momento.</div>
      )}

      {groups?.map(([source, sourceItems]) => (
        <section key={source} className="mb-5">
          <h2 className="h5 fw-bold border-bottom border-primary border-2 pb-2 mb-3">
            {source}
          </h2>
          <div className="row g-3">
            {sourceItems.map((item) => (
              <div key={item.link} className="col-md-6">
                <article className="card h-100">
                  <div className="card-body">
                    <h3 className="h6 fw-semibold mb-2">
                      <a
                        href={item.link}
                        target="_blank"
                        rel="noreferrer"
                        className="text-reset text-decoration-none stretched-link"
                      >
                        {item.title}
                      </a>
                    </h3>
                    {item.published_at && (
                      <p className="card-text small text-muted mb-0">{item.published_at}</p>
                    )}
                  </div>
                </article>
              </div>
            ))}
          </div>
        </section>
      ))}
    </div>
  )
}
