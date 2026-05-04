export default function PoliciesPage() {
  return (
    <div className="container py-4 py-md-5">
      <h1 className="fw-bold mb-3">Políticas de seguridad</h1>
      <p className="text-muted mb-4">
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
    </div>
  )
}
