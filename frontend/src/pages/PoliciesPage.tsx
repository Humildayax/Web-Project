export default function PoliciesPage() {
  return (
    <section>
      <h1>Políticas de seguridad</h1>
      <p className="card-meta" style={{ marginBottom: '1rem' }}>
        Si el visor no carga el documento, podés{' '}
        <a href="/policies.pdf" target="_blank" rel="noreferrer">
          descargarlo aquí
        </a>
        .
      </p>
      <iframe
        src="/policies.pdf"
        title="Políticas de seguridad"
        className="pdf-frame"
      />
    </section>
  )
}
