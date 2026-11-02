import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { AuthProvider } from './auth'
import { Admin } from './pages/Admin'
import { DocPage } from './pages/DocPage'
import { Home } from './pages/Home'
import { PageShell } from './components/PageShell'

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/docs/:slug/*" element={<DocPage />} />
          <Route path="/admin/*" element={<Admin />} />
          <Route
            path="*"
            element={
              <PageShell>
                <h1 className="article-title">404</h1>
                <div className="article-content">
                  <p>页面不存在。</p>
                </div>
              </PageShell>
            }
          />
        </Routes>
      </BrowserRouter>
    </AuthProvider>
  )
}
