import { useEffect, useRef, useState } from 'react'
import type { SearchTarget } from '../types'

interface Props {
  target: SearchTarget
}

export function NavSearch({ target }: Props) {
  const [open, setOpen] = useState(false)
  const [keyword, setKeyword] = useState('')
  const inputRef = useRef<HTMLInputElement>(null)
  const rootRef = useRef<HTMLFormElement>(null)

  useEffect(() => {
    if (open) inputRef.current?.focus()
  }, [open])

  useEffect(() => {
    if (!open) return
    const close = (event: PointerEvent) => {
      if (!rootRef.current?.contains(event.target as Node)) setOpen(false)
    }
    document.addEventListener('pointerdown', close)
    return () => document.removeEventListener('pointerdown', close)
  }, [open])

  const search = () => {
    const value = keyword.trim()
    if (!value) {
      setOpen(false)
      return
    }
    window.open(`${target.url}?q=${encodeURIComponent(value)}`, '_blank', 'noopener,noreferrer')
  }

  return (
    <form
      ref={rootRef}
      className={`nav-search${open ? ' open' : ''}`}
      role="search"
      onSubmit={(event) => {
        event.preventDefault()
        if (!open) {
          setOpen(true)
          return
        }
        search()
      }}
      onBlur={(event) => {
        if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setOpen(false)
      }}
    >
      <input
        ref={inputRef}
        className="nav-search-input"
        type="search"
        value={keyword}
        placeholder={`搜索 ${target.name}`}
        aria-label={`在 ${target.name} 的 Git 仓库中搜索`}
        tabIndex={open ? 0 : -1}
        onChange={(event) => setKeyword(event.target.value)}
        onKeyDown={(event) => {
          if (event.key !== 'Escape') return
          setOpen(false)
          event.currentTarget.blur()
        }}
      />
      <button type="submit" className="nav-search-submit" title="在 Git 仓库中搜索" aria-label="在 Git 仓库中搜索">
        <i className="fas fa-search" />
      </button>
    </form>
  )
}
