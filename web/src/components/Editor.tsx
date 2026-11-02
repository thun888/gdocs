import { useEffect, useRef } from 'react'
import { api } from '../api'
import { renderMathIn } from '../markdown'
import { isDark } from '../theme'

interface Props {
  value: string
  onChange: (value: string) => void
}

export function Editor({ value, onChange }: Props) {
  const hostRef = useRef<HTMLDivElement>(null)
  const cherryRef = useRef<{ destroy: () => void } | null>(null)
  const changeRef = useRef(onChange)
  changeRef.current = onChange

  useEffect(() => {
    if (!window.Cherry || !hostRef.current) return
    const id = `cherry-${Math.random().toString(36).slice(2)}`
    hostRef.current.id = id

    let mathTimer = 0
    const scheduleMath = () => {
      window.clearTimeout(mathTimer)
      mathTimer = window.setTimeout(() => {
        if (hostRef.current) renderMathIn(hostRef.current)
      }, 250)
    }

    const cherry = new window.Cherry({
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

    return () => {
      window.clearTimeout(mathTimer)
      cherryRef.current = null
      cherry.destroy()
    }
    // 编辑器只在挂载时创建一次，内容变更通过受控回调同步
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [])

  return <div ref={hostRef} className="cherry-host" />
}
