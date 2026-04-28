import { Link, NavLink, Outlet } from 'react-router-dom'

const navClass = ({ isActive }: { isActive: boolean }) =>
  isActive ? 'nav-link nav-link-active' : 'nav-link'

export default function Layout() {
  return (
    <div className="app">
      <header className="header">
        <Link to="/" className="brand">
          {/*
            Espacio del logo de la empresa.
            Para reemplazarlo: dropea el archivo en frontend/public/logo.svg
            y cambiá el <div> por:
              <img src="/logo.svg" alt="Empresa" className="logo-img" />
          */}
          <div className="logo-slot" aria-hidden>LOGO</div>
          <span className="brand-name">Security Portal</span>
        </Link>

        <nav className="nav">
          <NavLink to="/" end className={navClass}>Noticias</NavLink>
          <NavLink to="/report" className={navClass}>Reportar</NavLink>
          <NavLink to="/policies" className={navClass}>Políticas</NavLink>
        </nav>
      </header>

      <main className="container">
        <Outlet />
      </main>
    </div>
  )
}
