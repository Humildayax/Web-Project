import { Link, NavLink, Outlet } from 'react-router-dom'

const navLinkClass = ({ isActive }: { isActive: boolean }) =>
  isActive ? 'nav-link active fw-semibold' : 'nav-link'

export default function Layout() {
  return (
    <div className="d-flex flex-column min-vh-100 bg-body">
      <nav className="navbar navbar-expand-lg navbar-light bg-white border-bottom shadow-sm sticky-top">
        <div className="container">
          <Link to="/" className="navbar-brand d-flex align-items-center gap-2">
            <img src="/logo.svg" alt="" aria-hidden="true" className="brand-logo" />
            <span className="fw-semibold text-dark">Security Portal</span>
          </Link>

          <button
            className="navbar-toggler"
            type="button"
            data-bs-toggle="collapse"
            data-bs-target="#mainNav"
            aria-controls="mainNav"
            aria-expanded="false"
            aria-label="Toggle navigation"
          >
            <span className="navbar-toggler-icon" />
          </button>

          <div className="collapse navbar-collapse" id="mainNav">
            <ul className="navbar-nav ms-auto mb-2 mb-lg-0">
              <li className="nav-item">
                <NavLink to="/" end className={navLinkClass}>Inicio</NavLink>
              </li>
              <li className="nav-item">
                <NavLink to="/news" className={navLinkClass}>Noticias</NavLink>
              </li>
              <li className="nav-item">
                <NavLink to="/report" className={navLinkClass}>Reportar</NavLink>
              </li>
              <li className="nav-item">
                <NavLink to="/policies" className={navLinkClass}>Políticas</NavLink>
              </li>
            </ul>
          </div>
        </div>
      </nav>

      <main className="flex-grow-1">
        <Outlet />
      </main>

      <footer className="bg-dark text-white-50 py-3 mt-auto">
        <div className="container d-flex flex-column flex-md-row justify-content-between gap-2">
          <span>© {new Date().getFullYear()} Security Portal</span>
          <span>
            ¿Sospechas un incidente?{' '}
            <Link to="/report" className="link-light text-decoration-underline">
              Reportar ahora
            </Link>
          </span>
        </div>
      </footer>
    </div>
  )
}
