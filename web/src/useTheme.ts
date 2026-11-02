import { useEffect, useState } from 'react'
import { isDark } from './theme'

export function useTheme(): boolean {
  const [dark, setDark] = useState(isDark())

  useEffect(() => {
    const observer = new MutationObserver(() => setDark(isDark()))
    observer.observe(document.body, { attributes: true, attributeFilter: ['class'] })
    return () => observer.disconnect()
  }, [])

  return dark
}
