import { useEffect, useMemo, useRef } from 'react'
import { enhance, renderMarkdown } from '../markdown'

export function MarkdownView({ content, dark }: { content: string; dark: boolean }) {
  const ref = useRef<HTMLDivElement>(null)
  const html = useMemo(() => renderMarkdown(content), [content, dark])

  useEffect(() => {
    const element = ref.current
    if (!element) return
    let cancelled = false
    void enhance(element, dark).then(() => {
      if (cancelled) return
    })
    return () => {
      cancelled = true
    }
  }, [html, dark])

  return <div className="article-content" ref={ref} dangerouslySetInnerHTML={{ __html: html }} />
}
