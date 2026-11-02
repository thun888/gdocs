import DOMPurify from 'dompurify'
import hljs from 'highlight.js/lib/common'
import mermaid from 'mermaid'

declare global {
  interface Window {
    Cherry?: new (options: Record<string, unknown>) => {
      engine: { makeHtml: (text: string) => string }
      destroy: () => void
    }
  }
}

let engine: ReturnType<NonNullable<Window['Cherry']>> | null = null

function cherry() {
  if (!engine && window.Cherry) {
    engine = new window.Cherry({
      engine: {
        global: {
          classicBr: false,
        },
      },
    })
  }
  return engine
}

export function renderMarkdown(markdown: string): string {
  const instance = cherry()
  const raw = instance ? instance.engine.makeHtml(markdown) : markdown
  return DOMPurify.sanitize(raw, {
    FORBID_TAGS: ['script', 'iframe', 'object', 'embed', 'form', 'input', 'button'],
    FORBID_ATTR: ['onerror', 'onclick', 'onload', 'onmouseover'],
  })
}

let mermaidReady = false

export async function enhance(root: HTMLElement, dark: boolean): Promise<void> {
  if (!mermaidReady) {
    mermaid.initialize({ startOnLoad: false, securityLevel: 'strict' })
    mermaidReady = true
  }
  mermaid.initialize({ startOnLoad: false, securityLevel: 'strict', theme: dark ? 'dark' : 'default' })

  const blocks = Array.from(root.querySelectorAll<HTMLElement>('pre > code.language-mermaid'))
  for (const code of blocks) {
    const source = code.textContent ?? ''
    const id = `mermaid-${Math.random().toString(36).slice(2)}`
    try {
      const { svg } = await mermaid.render(id, source)
      const wrapper = document.createElement('div')
      wrapper.className = 'mermaid-block'
      wrapper.innerHTML = svg
      code.parentElement?.replaceWith(wrapper)
    } catch {
      code.parentElement?.classList.add('mermaid-error')
    }
  }

  root.querySelectorAll<HTMLElement>('pre code').forEach((block) => {
    if (block.classList.contains('language-mermaid') || block.classList.contains('hljs')) return
    try {
      hljs.highlightElement(block)
    } catch {
      /* 忽略未知语言 */
    }
  })
}
