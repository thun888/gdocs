import { useEffect, useState } from 'react'

const STORAGE_KEY = 'gdocs-font-scale'
const STEPS = [0.8, 0.9, 1, 1.1, 1.2, 1.3]

function readScale(): number {
  const saved = Number(window.localStorage.getItem(STORAGE_KEY))
  return STEPS.includes(saved) ? saved : 1
}

export function useFontScale() {
  const [scale, setScale] = useState(readScale)

  useEffect(() => {
    if (scale === 1) {
      document.documentElement.style.removeProperty('--font-scale')
      window.localStorage.removeItem(STORAGE_KEY)
    } else {
      document.documentElement.style.setProperty('--font-scale', String(scale))
      window.localStorage.setItem(STORAGE_KEY, String(scale))
    }
  }, [scale])

  const index = STEPS.indexOf(scale)
  return {
    scale,
    larger: () => setScale(STEPS[Math.min(index + 1, STEPS.length - 1)]),
    smaller: () => setScale(STEPS[Math.max(index - 1, 0)]),
    canGrow: index < STEPS.length - 1,
    canShrink: index > 0,
  }
}
