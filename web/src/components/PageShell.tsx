import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import { useNavigate } from 'react-router-dom'
import { loadSite, type SiteInfo } from '../site'
import { useAuth } from '../auth'
import { toggleTheme } from '../theme'

interface Props {
  breadcrumb?: ReactNode
  sidebar?: ReactNode
  children: ReactNode
}

export function PageShell({ breadcrumb, sidebar, children }: Props) {
  const [site, setSite] = useState<SiteInfo>({ name: 'ZDocs', description: '', public_read: true })
  const { user, login, logout } = useAuth()
  const navigate = useNavigate()

  useEffect(() => {
    loadSite().then(setSite).catch(() => undefined)
  }, [])

  useEffect(() => {
    document.title = site.name
  }, [site.name])

  return (
    <>
      <div className="switch-theme" title="切换主题" onClick={() => toggleTheme()}>
        <i className="fas fa-sun" />
        <i className="fas fa-moon" />
      </div>
      <div className="main-container row">
        <div className="navbar">
          <span className="menu">
            <i className="fas fa-bars" />
          </span>
          <span className="logo" onClick={() => navigate('/')} style={{ cursor: 'pointer' }}>
            {site.name}
          </span>
          {breadcrumb && <span className="page-breadcrumb">{breadcrumb}</span>}
          <span className="login">
            {user ? (
              <>
                <span className="login-info">
                  <a onClick={() => navigate('/admin')}>
                    <span className="login-avatar initial">{user.avatar}</span> {user.display_name}
                  </a>
                </span>
                <a onClick={() => void logout()}>
                  <i className="fas fa-sign-out-alt" />
                  退出
                </a>
              </>
            ) : (
              <a onClick={login}>
                <b>
                  <i className="fas fa-sign-in-alt" />
                  登录账户
                </b>
              </a>
            )}
          </span>
        </div>
        {sidebar}
        <div className="main-articles">{children}</div>
      </div>
    </>
  )
}
