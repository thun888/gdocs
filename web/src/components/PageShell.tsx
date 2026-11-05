import { useEffect, useState } from 'react'
import type { ReactNode } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { loadSite, type SiteInfo } from '../site'
import { useAuth } from '../auth'
import { toggleTheme } from '../theme'
import { useFontScale } from '../useFontScale'
import { NavSearch } from './NavSearch'

interface Props {
  breadcrumb?: ReactNode
  sidebar?: ReactNode
  children: ReactNode
  onDropFiles?: (files: File[]) => void
}

export function PageShell({ breadcrumb, sidebar, children, onDropFiles }: Props) {
  const [site, setSite] = useState<SiteInfo>({
    name: 'gdocs',
    description: '',
    public_read: true,
    header_show_avatar: true,
    header_show_name: true,
    header_show_logout: true,
    search: [],
  })
  const [menuOpen, setMenuOpen] = useState(false)
  const { user, login, logout } = useAuth()
  const navigate = useNavigate()
  const { slug } = useParams()
  const font = useFontScale()
  const searchTarget = slug ? site.search?.find((item) => item.slug === slug) : undefined

  useEffect(() => {
    loadSite().then(setSite).catch(() => undefined)
  }, [])

  useEffect(() => {
    document.title = site.name
  }, [site.name])

  return (
    <>
      <div className={`main-container${sidebar ? ' has-sidebar' : ''}${menuOpen ? ' nav-open' : ''}`}>
        <div className="navbar">
          <button
            type="button"
            className="menu"
            aria-label="切换目录"
            aria-expanded={menuOpen}
            onClick={() => setMenuOpen((open) => !open)}
          >
            <i className="fas fa-bars" />
          </button>
          <span
            className="logo"
            role="link"
            tabIndex={0}
            onClick={() => navigate('/')}
            onKeyDown={(event) => {
              if (event.key !== 'Enter') return
              event.preventDefault()
              navigate('/')
            }}
          >
            {site.name}
          </span>
          {breadcrumb && <span className="page-breadcrumb">{breadcrumb}</span>}
          <div className="side-tools">
            <button
              type="button"
              className="switch-theme"
              title="切换主题"
              aria-label="切换主题"
              onClick={() => toggleTheme()}
            >
              <i className="fas fa-sun" />
              <i className="fas fa-moon" />
            </button>
            <div className="font-size-control" role="group" aria-label="字号">
              <button onClick={font.larger} disabled={!font.canGrow} title="放大字号">
                A+
              </button>
              <button onClick={font.smaller} disabled={!font.canShrink} title="缩小字号">
                A−
              </button>
            </div>
          </div>
          {searchTarget && <NavSearch target={searchTarget} />}
          <span className="login">
            {user ? (
              <>
                {(site.header_show_avatar || site.header_show_name) && (
                  <span className="login-info">
                    <a onClick={() => navigate('/admin')}>
                      {site.header_show_avatar && <span className="login-avatar initial">{user.avatar}</span>}
                      {site.header_show_name && <span className="login-name">{user.display_name}</span>}
                    </a>
                  </span>
                )}
                {site.header_show_logout && (
                  <a onClick={() => void logout()}>
                    <i className="fas fa-sign-out-alt" />
                    退出
                  </a>
                )}
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
        <div
          className="main-articles"
          onDragOver={(event) => {
            if (onDropFiles) event.preventDefault()
          }}
          onDrop={(event) => {
            if (!onDropFiles) return
            event.preventDefault()
            const files = Array.from(event.dataTransfer.files).filter((file) => /\.md$/i.test(file.name))
            if (files.length > 0) onDropFiles(files)
          }}
        >
          {children}
        </div>
      </div>
    </>
  )
}
