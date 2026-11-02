import DOMPurify from 'dompurify'
import hljs from 'highlight.js/lib/common'
import katex from 'katex'
import mermaid from 'mermaid'
import 'katex/dist/katex.min.css'

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
    // Cherry 构造时会创建隐藏 textarea 并聚焦，浏览器为露出它会把页面向上滚，
    // 导致标题被固定顶栏遮住。只在构造期间屏蔽 textarea 聚焦，不影响编辑器。
    const proto = HTMLTextAreaElement.prototype
    const originalFocus = proto.focus
    proto.focus = function () {}
    try {
      engine = new window.Cherry({
        engine: {
          global: {
            classicBr: false,
          },
        },
      })
    } finally {
      proto.focus = originalFocus
    }
  }
  return engine
}

export function stripLeadingTitle(markdown: string, title: string): string {
  const lines = markdown.split('\n')
  const index = lines.findIndex((line) => line.trim() !== '')
  if (index < 0) return markdown

  const match = lines[index].match(/^#\s+(.+?)\s*#*\s*$/)
  if (!match) return markdown

  const heading = match[1].replace(/\s+#+$/, '').trim()
  if (heading !== title.trim()) return markdown

  lines.splice(index, 1)
  return lines.join('\n')
}

export function renderMarkdown(markdown: string): string {  const instance = cherry()
  const raw = instance ? instance.engine.makeHtml(markdown) : markdown
  return DOMPurify.sanitize(raw, {
    FORBID_TAGS: ['script', 'iframe', 'object', 'embed', 'form', 'input', 'button'],
    FORBID_ATTR: ['onerror', 'onclick', 'onload', 'onmouseover'],
  })
}

let mermaidReady = false

function texSource(raw: string): string {
  const value = raw.trim()
  const pairs: [string, string][] = [
    ['$$', '$$'],
    ['\\[', '\\]'],
    ['\\(', '\\)'],
    ['$', '$'],
  ]
  for (const [open, close] of pairs) {
    if (value.startsWith(open) && value.endsWith(close) && value.length > open.length + close.length) {
      return value.slice(open.length, value.length - close.length).trim()
    }
  }
  return value
}

export function renderMathIn(root: HTMLElement): void {
  const nodes = root.querySelectorAll<HTMLElement>('.Cherry-InlineMath:not([data-math-done]), .Cherry-Math:not([data-math-done])')
  nodes.forEach((node) => {
    const source = node.textContent ?? ''
    if (!source.trim()) return
    node.dataset.mathDone = '1'
    const displayMode = node.classList.contains('Cherry-Math') || source.trim().startsWith('$$')
    try {
      katex.render(texSource(source), node, {
        displayMode,
        throwOnError: false,
        strict: false,
        trust: false,
      })
    } catch {
      /* 公式语法错误时保留原文 */
    }
  })
}

export async function enhance(root: HTMLElement, dark: boolean): Promise<void> {
  renderMathIn(root)

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
