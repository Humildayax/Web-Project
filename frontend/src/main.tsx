import { StrictMode } from 'react'
import { createRoot } from 'react-dom/client'
import App from './App'
// Bootstrap CSS antes que index.css para que nuestros overrides ganen.
// El bundle JS solo lo importamos para los componentes que necesiten data-bs-*
// (collapse del navbar móvil). Sin él, la hamburguesa no expande.
import 'bootstrap/dist/css/bootstrap.min.css'
import 'bootstrap/dist/js/bootstrap.bundle.min.js'
import './index.css'

const rootElement = document.getElementById('root')
if (!rootElement) throw new Error('No se encontró el elemento #root en index.html')

createRoot(rootElement).render(
  <StrictMode>
    <App />
  </StrictMode>,
)
