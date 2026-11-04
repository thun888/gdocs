import { useEffect, useRef, useState } from 'react'
import { enhance, renderMarkdown } from '../markdown'

interface Props {
  content: string
  dark: boolean
  onRendered?: () => void
}

export function MarkdownView({ content, dark, onRendered }: Props) {
  const ref = useRef<HTMLDivElement>(null)
  const [html, setHtml] = useState('')

  useEffect(() => {
    let cancelled = false
    void renderMarkdown(content)
      .then((next) => {
        if (!cancelled) setHtml(next)
      })
      .catch(() => undefined)
    return () => {
      cancelled = true
    }
  }, [content])

  useEffect(() => {
    if (html) onRendered?.()
  }, [html, onRendered])

  useEffect(() => {
    const element = ref.current
    if (!element || !html) return
    let cancelled = false
    void enhance(element, dark)
      .then(() => {
        if (cancelled) return
      })
      .catch(() => undefined)
    return () => {
      cancelled = true
    }
  }, [html, dark])

  return <div className="article-content" ref={ref} dangerouslySetInnerHTML={{ __html: html }} />
}
