import { createContext, useCallback, useContext, useEffect, useMemo, useState } from 'react'
import type { ReactNode } from 'react'
import { api, setCSRF } from './api'
import type { Group, MeResponse, User } from './types'

interface AuthState {
  user: User | null
  groups: Group[]
  csrf: string
  publicRead: boolean
  ready: boolean
  canEdit: boolean
  isAdmin: boolean
  refresh: () => Promise<void>
  login: () => void
  logout: () => Promise<void>
}

const AuthContext = createContext<AuthState | null>(null)

export function AuthProvider({ children }: { children: ReactNode }) {
  const [user, setUser] = useState<User | null>(null)
  const [groups, setGroups] = useState<Group[]>([])
  const [csrf, setCsrf] = useState('')
  const [publicRead, setPublicRead] = useState(true)
  const [ready, setReady] = useState(false)

  const refresh = useCallback(async () => {
    try {
      const me = await api.get<MeResponse>('/me')
      setUser(me.user)
      setGroups(me.groups ?? [])
      setPublicRead(me.public_read)
      if (me.csrf) {
        setCsrf(me.csrf)
        setCSRF(me.csrf)
      }
    } catch {
      setUser(null)
    } finally {
      setReady(true)
    }
  }, [])

  useEffect(() => {
    void refresh()
  }, [refresh])

  const login = useCallback(() => {
    const redirect = window.location.pathname + window.location.search
    window.location.href = `/api/v1/auth/login?redirect=${encodeURIComponent(redirect)}`
  }, [])

  const logout = useCallback(async () => {
    await api.post('/auth/logout')
    setUser(null)
    setGroups([])
    setCsrf('')
    setCSRF('')
    await refresh()
  }, [refresh])

  const value = useMemo<AuthState>(
    () => ({
      user,
      groups,
      csrf,
      publicRead,
      ready,
      canEdit: user?.role === 'editor' || user?.role === 'admin',
      isAdmin: user?.role === 'admin',
      refresh,
      login,
      logout,
    }),
    [user, groups, csrf, publicRead, ready, refresh, login, logout],
  )

  return <AuthContext.Provider value={value}>{children}</AuthContext.Provider>
}

export function useAuth(): AuthState {
  const ctx = useContext(AuthContext)
  if (!ctx) throw new Error('AuthProvider 未挂载')
  return ctx
}
