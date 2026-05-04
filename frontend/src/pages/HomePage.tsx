import { Link } from 'react-router-dom'

// Iconos SVG inline simples — evita sumar otra dependencia (Bootstrap Icons).
// `currentColor` los hace heredar el color del padre.
const NewsIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" fill="currentColor" viewBox="0 0 16 16" aria-hidden="true">
    <path d="M5 4a.5.5 0 0 0 0 1h6a.5.5 0 0 0 0-1H5zm-.5 2.5A.5.5 0 0 1 5 6h6a.5.5 0 0 1 0 1H5a.5.5 0 0 1-.5-.5zM5 8a.5.5 0 0 0 0 1h6a.5.5 0 0 0 0-1H5zm0 2a.5.5 0 0 0 0 1h3a.5.5 0 0 0 0-1H5z" />
    <path d="M2 2a2 2 0 0 1 2-2h8a2 2 0 0 1 2 2v12a2 2 0 0 1-2 2H4a2 2 0 0 1-2-2V2zm10-1H4a1 1 0 0 0-1 1v12a1 1 0 0 0 1 1h8a1 1 0 0 0 1-1V2a1 1 0 0 0-1-1z" />
  </svg>
)
const ReportIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" fill="currentColor" viewBox="0 0 16 16" aria-hidden="true">
    <path d="M8.982 1.566a1.13 1.13 0 0 0-1.96 0L.165 13.233c-.457.778.091 1.767.98 1.767h13.713c.889 0 1.438-.99.98-1.767L8.982 1.566zM8 5c.535 0 .954.462.9.995l-.35 3.507a.552.552 0 0 1-1.1 0L7.1 5.995A.905.905 0 0 1 8 5zm.002 6a1 1 0 1 1 0 2 1 1 0 0 1 0-2z" />
  </svg>
)
const DocIcon = () => (
  <svg xmlns="http://www.w3.org/2000/svg" width="32" height="32" fill="currentColor" viewBox="0 0 16 16" aria-hidden="true">
    <path d="M5 4a.5.5 0 0 1 .5-.5h5a.5.5 0 0 1 0 1h-5A.5.5 0 0 1 5 4zm-.5 2.5A.5.5 0 0 1 5 6h5a.5.5 0 0 1 0 1H5a.5.5 0 0 1-.5-.5zM5 8a.5.5 0 0 0 0 1h5a.5.5 0 0 0 0-1H5zm0 2a.5.5 0 0 0 0 1h2a.5.5 0 0 0 0-1H5z" />
    <path d="M9.5 0H4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2V4.5L9.5 0zM3 2a1 1 0 0 1 1-1h5v3a1 1 0 0 0 1 1h3v9a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V2z" />
  </svg>
)

export default function HomePage() {
  return (
    <>
      {/* HERO */}
      <section className="hero-dark py-5">
        <div className="container py-4 text-center text-md-start">
          <div className="row align-items-center g-4">
            <div className="col-md-8">
              <h1 className="display-5 fw-bold mb-3">Security Portal</h1>
              <p className="lead text-white-50 mb-4">
                Centro de ciberseguridad: reportá incidentes, consultá las
                últimas amenazas y revisá las políticas vigentes en un solo lugar.
              </p>
              <div className="d-flex gap-2 flex-wrap justify-content-center justify-content-md-start">
                <Link to="/report" className="btn btn-primary btn-lg">
                  Reportar incidente
                </Link>
                <Link to="/news" className="btn btn-outline-light btn-lg">
                  Ver noticias
                </Link>
              </div>
            </div>
            <div className="col-md-4 text-center">
              <img
                src="/logo.svg"
                alt=""
                aria-hidden="true"
                className="img-fluid"
                style={{ maxHeight: '120px', filter: 'brightness(0) invert(1)' }}
              />
            </div>
          </div>
        </div>
      </section>

      {/* QUÉ HACER ANTE UN INCIDENTE */}
      <section className="py-5 bg-light">
        <div className="container">
          <div className="text-center mb-5">
            <h2 className="fw-bold">¿Qué hacer ante un incidente?</h2>
            <p className="text-muted">Cuatro pasos rápidos para minimizar el impacto.</p>
          </div>
          <div className="row g-4">
            <div className="col-md-6 col-lg-3">
              <div className="d-flex">
                <span className="badge bg-primary fs-5 me-3 align-self-start">1</span>
                <div>
                  <h5 className="fw-semibold mb-1">No interactúes</h5>
                  <p className="text-muted small mb-0">
                    No abras enlaces, archivos adjuntos ni respondas mensajes sospechosos.
                  </p>
                </div>
              </div>
            </div>
            <div className="col-md-6 col-lg-3">
              <div className="d-flex">
                <span className="badge bg-primary fs-5 me-3 align-self-start">2</span>
                <div>
                  <h5 className="fw-semibold mb-1">Conservá la evidencia</h5>
                  <p className="text-muted small mb-0">
                    Tomá capturas, no borres correos ni mensajes; los necesita el equipo
                    de seguridad.
                  </p>
                </div>
              </div>
            </div>
            <div className="col-md-6 col-lg-3">
              <div className="d-flex">
                <span className="badge bg-primary fs-5 me-3 align-self-start">3</span>
                <div>
                  <h5 className="fw-semibold mb-1">Reportá acá</h5>
                  <p className="text-muted small mb-0">
                    Llenando el formulario se abre un ticket interno y la evidencia
                    queda registrada.
                  </p>
                </div>
              </div>
            </div>
            <div className="col-md-6 col-lg-3">
              <div className="d-flex">
                <span className="badge bg-primary fs-5 me-3 align-self-start">4</span>
                <div>
                  <h5 className="fw-semibold mb-1">Esperá indicaciones</h5>
                  <p className="text-muted small mb-0">
                    El equipo se contacta vía los canales oficiales. Nunca pedimos
                    contraseñas ni códigos por correo.
                  </p>
                </div>
              </div>
            </div>
          </div>
        </div>
      </section>

      {/* TRES CARDS DE NAVEGACIÓN */}
      <section className="py-5">
        <div className="container">
          <div className="text-center mb-5">
            <h2 className="fw-bold">Recursos disponibles</h2>
          </div>
          <div className="row g-4">
            <div className="col-md-4">
              <Link to="/news" className="text-decoration-none text-reset">
                <div className="card nav-card h-100">
                  <div className="card-body p-4">
                    <div className="text-primary mb-3"><NewsIcon /></div>
                    <h5 className="card-title fw-semibold">Noticias de seguridad</h5>
                    <p className="card-text text-muted">
                      Últimas alertas, vulnerabilidades y campañas activas — actualizado
                      desde fuentes especializadas.
                    </p>
                    <span className="text-primary fw-semibold">Ver noticias →</span>
                  </div>
                </div>
              </Link>
            </div>
            <div className="col-md-4">
              <Link to="/report" className="text-decoration-none text-reset">
                <div className="card nav-card h-100">
                  <div className="card-body p-4">
                    <div className="text-primary mb-3"><ReportIcon /></div>
                    <h5 className="card-title fw-semibold">Reportar incidente</h5>
                    <p className="card-text text-muted">
                      Formulario directo al equipo de seguridad. El reporte queda
                      registrado aunque JIRA esté caído.
                    </p>
                    <span className="text-primary fw-semibold">Reportar ahora →</span>
                  </div>
                </div>
              </Link>
            </div>
            <div className="col-md-4">
              <Link to="/policies" className="text-decoration-none text-reset">
                <div className="card nav-card h-100">
                  <div className="card-body p-4">
                    <div className="text-primary mb-3"><DocIcon /></div>
                    <h5 className="card-title fw-semibold">Políticas corporativas</h5>
                    <p className="card-text text-muted">
                      Documento oficial de uso aceptable, manejo de información y
                      gobierno de IA.
                    </p>
                    <span className="text-primary fw-semibold">Leer políticas →</span>
                  </div>
                </div>
              </Link>
            </div>
          </div>
        </div>
      </section>

      {/* CIERRE / CONTACTO */}
      <section className="py-5 bg-dark text-white">
        <div className="container text-center">
          <h3 className="fw-bold mb-3">¿Tienes dudas sobre seguridad?</h3>
          <p className="text-white-50 mb-4">
            Si no estás seguro de si algo es un incidente, reportalo igual. Es preferible
            un falso positivo que un compromiso pasado por alto.
          </p>
          <Link to="/report" className="btn btn-primary btn-lg">
            Reportar incidente
          </Link>
        </div>
      </section>
    </>
  )
}
