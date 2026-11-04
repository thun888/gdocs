import DOMPurify from 'dompurify'
import { ensureCherry, type CherryInstance } from './loader'

let engine: CherryInstance | null = null

function cherry(): CherryInstance | null {
  if (!engine && window.Cherry) {
    // Cherry 构造时会创建隐藏 textarea 并聚焦，浏览器为露出它会把页面向上滚，
    // 导致标题被固定顶栏遮住。只在构造期间屏蔽 textarea 聚焦，不影响编辑器。
    const proto = HTMLTextAreaElement.prototype
    const originalFocus = proto.focus
    proto.focus = function () {}
    try {
      // 只借用渲染引擎，挂到游离节点上，避免 Cherry 往 body 末尾插入编辑器界面
      engine = new window.Cherry({
        el: document.createElement('div'),
        forceAppend: false,
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

export async function renderMarkdown(markdown: string): Promise<string> {
  try {
    await ensureCherry()
  } catch {
    /* 渲染器不可用时退回原文，由调用方保证内容可读 */
  }
  const instance = cherry()
  const raw = instance ? instance.engine.makeHtml(markdown) : markdown
  return DOMPurify.sanitize(raw, {
    FORBID_TAGS: ['script', 'iframe', 'object', 'embed', 'form', 'input', 'button'],
    FORBID_ATTR: ['onerror', 'onclick', 'onload', 'onmouseover'],
  })
}

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

export async function renderMathIn(root: HTMLElement): Promise<void> {
  const nodes = root.querySelectorAll<HTMLElement>(
    '.Cherry-InlineMath:not([data-math-done]), .Cherry-Math:not([data-math-done])',
  )
  if (nodes.length === 0) return

  const [{ default: katex }] = await Promise.all([import('katex'), import('./katexStyles')])
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

async function renderDiagrams(root: HTMLElement, dark: boolean): Promise<void> {
  const blocks = Array.from(root.querySelectorAll<HTMLElement>('pre > code.language-mermaid'))
  if (blocks.length === 0) return

  const { default: mermaid } = await import('mermaid')
  mermaid.initialize({ startOnLoad: false, securityLevel: 'strict', theme: dark ? 'dark' : 'default' })

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
}

async function highlightCode(root: HTMLElement): Promise<void> {
  const blocks = Array.from(root.querySelectorAll<HTMLElement>('pre code')).filter(
    (block) => !block.classList.contains('language-mermaid') && !block.classList.contains('hljs'),
  )
  if (blocks.length === 0) return

  const { default: hljs } = await import('highlight.js/lib/common')
  for (const block of blocks) {
    try {
      hljs.highlightElement(block)
    } catch {
      /* 忽略未知语言 */
    }
  }
}

export async function enhance(root: HTMLElement, dark: boolean): Promise<void> {
  await Promise.all([renderMathIn(root), renderDiagrams(root, dark), highlightCode(root)])
}
