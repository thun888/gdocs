const STORAGE_KEY = 'zdocs-theme'

export function isDark(): boolean {
  return document.body.classList.contains('theme-dark')
}

export function applyTheme(dark: boolean) {
  document.body.classList.toggle('theme-dark', dark)
  const meta = document.querySelector('meta[name="theme-color"]')
  if (meta) meta.setAttribute('content', dark ? '#1e212e' : '#FFFFFF')
  localStorage.setItem(STORAGE_KEY, dark ? 'dark' : 'light')
}

export function toggleTheme(): boolean {
  const next = !isDark()
  applyTheme(next)
  return next
}
