import { useRef, useState } from 'react'

export const MIN_CONTENT_WIDTH = 560

export function maxContentWidth(): number {
  const main = document.querySelector<HTMLElement>('.main-articles')
  const available = main ? main.clientWidth - 80 : window.innerWidth - 380
  return Math.max(MIN_CONTENT_WIDTH, Math.round(available))
}

export function clampContentWidth(width: number): number {
  return Math.min(Math.max(Math.round(width), MIN_CONTENT_WIDTH), maxContentWidth())
}

interface Props {
  width: number | null
  onChange: (width: number | null) => void
}

export function ContentResizer({ width, onChange }: Props) {
  const drag = useRef<{ x: number; start: number } | null>(null)
  const [dragging, setDragging] = useState(false)

  const current = () =>
    width ?? document.querySelector<HTMLElement>('.doc-body .article-title')?.getBoundingClientRect().width ?? 900

  const start = (event: React.PointerEvent<HTMLDivElement>) => {
    event.preventDefault()
    drag.current = { x: event.clientX, start: current() }
    setDragging(true)
    document.body.classList.add('is-resizing')
    event.currentTarget.setPointerCapture(event.pointerId)
  }

  const move = (event: React.PointerEvent<HTMLDivElement>) => {
    if (!drag.current) return
    onChange(clampContentWidth(drag.current.start + event.clientX - drag.current.x))
  }

  const end = (event: React.PointerEvent<HTMLDivElement>) => {
    if (!drag.current) return
    drag.current = null
    setDragging(false)
    document.body.classList.remove('is-resizing')
    event.currentTarget.releasePointerCapture(event.pointerId)
  }

  return (
    <div
      className={`content-resizer${dragging ? ' dragging' : ''}`}
      role="separator"
      aria-orientation="vertical"
      aria-label="调整内容宽度"
      aria-valuenow={Math.round(current())}
      tabIndex={0}
      title="拖动调整内容宽度，双击恢复默认"
      onPointerDown={start}
      onPointerMove={move}
      onPointerUp={end}
      onPointerCancel={end}
      onDoubleClick={() => onChange(null)}
      onKeyDown={(event) => {
        if (event.key !== 'ArrowLeft' && event.key !== 'ArrowRight') return
        event.preventDefault()
        onChange(clampContentWidth(current() + (event.key === 'ArrowRight' ? 24 : -24)))
      }}
    />
  )
}
