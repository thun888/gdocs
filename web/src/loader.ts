export interface CherryInstance {
  engine: { makeHtml: (text: string) => string }
  destroy: () => void
}

declare global {
  interface Window {
    Cherry?: new (options: Record<string, unknown>) => CherryInstance
  }
}

const scripts = new Map<string, Promise<void>>()
const styles = new Map<string, Promise<void>>()

export function loadScript(src: string): Promise<void> {
  const pending = scripts.get(src)
  if (pending) return pending
  const task = new Promise<void>((resolve, reject) => {
    const element = document.createElement('script')
    element.src = src
    element.async = true
    element.onload = () => resolve()
    element.onerror = () => {
      scripts.delete(src)
      element.remove()
      reject(new Error(`脚本加载失败：${src}`))
    }
    document.head.appendChild(element)
  })
  scripts.set(src, task)
  return task
}

export function loadStyle(href: string): Promise<void> {
  const pending = styles.get(href)
  if (pending) return pending
  const task = new Promise<void>((resolve, reject) => {
    const element = document.createElement('link')
    element.rel = 'stylesheet'
    element.href = href
    element.onload = () => resolve()
    element.onerror = () => {
      styles.delete(href)
      element.remove()
      reject(new Error(`样式加载失败：${href}`))
    }
    document.head.appendChild(element)
  })
  styles.set(href, task)
  return task
}

let cherryReady: Promise<void> | null = null

export function ensureCherry(): Promise<void> {
  if (window.Cherry) return Promise.resolve()
  if (!cherryReady) {
    cherryReady = Promise.all([
      loadStyle('/css/cherry-markdown.min.css'),
      loadScript('/js/cherry-markdown.min.js'),
    ])
      .then(() => undefined)
      .catch((error: unknown) => {
        cherryReady = null
        throw error
      })
  }
  return cherryReady
}
