import { useEffect, useState } from 'react'
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

  return (
    <section>
      <h1>Noticias de seguridad</h1>

      {error && <div className="alert alert-error">{error}</div>}

      {!items && !error && <p className="empty">Cargando…</p>}

      {items && items.length === 0 && (
        <p className="empty">No hay noticias por el momento.</p>
      )}

      {items?.map((item) => (
        <article key={item.link} className="card">
          <h3 className="card-title">
            <a href={item.link} target="_blank" rel="noreferrer">
              {item.title}
            </a>
          </h3>
          <p className="card-meta">
            {item.source} · {item.published_at}
          </p>
        </article>
      ))}
    </section>
  )
}
