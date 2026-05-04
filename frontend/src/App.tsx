import { BrowserRouter, Route, Routes } from 'react-router-dom'
import Layout from './components/Layout'
import HomePage from './pages/HomePage'
import NewsPage from './pages/NewsPage'
import ReportPage from './pages/ReportPage'
import PoliciesPage from './pages/PoliciesPage'

function App() {
  return (
    <BrowserRouter>
      <Routes>
        <Route element={<Layout />}>
          <Route index element={<HomePage />} />
          <Route path="/news" element={<NewsPage />} />
          <Route path="/report" element={<ReportPage />} />
          <Route path="/policies" element={<PoliciesPage />} />
        </Route>
      </Routes>
    </BrowserRouter>
  )
}

export default App
