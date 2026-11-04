import { Suspense, lazy } from 'react'
import { BrowserRouter, Route, Routes } from 'react-router-dom'
import { AuthProvider } from './auth'
import { PageShell } from './components/PageShell'

const Home = lazy(() => import('./pages/Home').then((module) => ({ default: module.Home })))
const DocPage = lazy(() => import('./pages/DocPage').then((module) => ({ default: module.DocPage })))
const Admin = lazy(() => import('./pages/Admin').then((module) => ({ default: module.Admin })))

export default function App() {
  return (
    <AuthProvider>
      <BrowserRouter>
        <Suspense fallback={<p className="route-loading">加载中…</p>}>
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
        </Suspense>
      </BrowserRouter>
    </AuthProvider>
  )
}
