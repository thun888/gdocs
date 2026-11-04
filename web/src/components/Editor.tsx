import { useEffect, useRef } from 'react'
import { api } from '../api'
import { ensureCherry, type CherryInstance } from '../loader'
import { renderMathIn } from '../markdown'
import { isDark } from '../theme'

interface Props {
  value: string
  onChange: (value: string) => void
}

export function Editor({ value, onChange }: Props) {
  const hostRef = useRef<HTMLDivElement>(null)
  const cherryRef = useRef<CherryInstance | null>(null)
  const changeRef = useRef(onChange)
  changeRef.current = onChange

  useEffect(() => {
    if (!hostRef.current) return
    let cancelled = false
    let cherry: CherryInstance | null = null
    let mathTimer = 0
    const scheduleMath = () => {
      window.clearTimeout(mathTimer)
      mathTimer = window.setTimeout(() => {
        if (hostRef.current) void renderMathIn(hostRef.current).catch(() => undefined)
      }, 250)
    }

    void ensureCherry()
      .then(() => {
        if (cancelled || !hostRef.current || !window.Cherry) return
        const id = `cherry-${Math.random().toString(36).slice(2)}`
        hostRef.current.id = id
        cherry = new window.Cherry({
          id,
          value,
          themeSettings: { mainTheme: isDark() ? 'dark' : 'light' },
          toolbars: { toc: false },
          callback: {
            afterChange: (text: string) => {
              changeRef.current(text)
              scheduleMath()
            },
          },
          fileUpload: (file: File, callback: (url: string) => void) => {
            api
              .upload(file)
              .then((result) => callback(result.url))
              .catch(() => callback(''))
          },
        })
        cherryRef.current = cherry
        scheduleMath()
      })
      .catch(() => undefined)

    return () => {
      cancelled = true
      window.clearTimeout(mathTimer)
      cherryRef.current = null
      cherry?.destroy()
    }
    // 编辑器只在挂载时创建一次，内容变更通过受控回调同步
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return <div ref={hostRef} className="cherry-host" />
}
