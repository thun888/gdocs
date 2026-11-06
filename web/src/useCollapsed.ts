import { useEffect, useState } from 'react'

export function useCollapsed(storageKey: string) {
  const [collapsed, setCollapsed] = useState(() => window.localStorage.getItem(storageKey) === '1')

  useEffect(() => {
    if (collapsed) window.localStorage.setItem(storageKey, '1')
    else window.localStorage.removeItem(storageKey)
  }, [collapsed, storageKey])

  return { collapsed, toggle: () => setCollapsed((value) => !value) }
}
