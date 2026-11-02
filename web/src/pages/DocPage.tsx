import { useCallback, useEffect, useMemo, useState } from 'react'
import { useNavigate, useParams } from 'react-router-dom'
import { ApiError, api } from '../api'
import { useAuth } from '../auth'
import { Editor } from '../components/Editor'
import { HistoryPanel } from '../components/HistoryPanel'
import { MarkdownView } from '../components/MarkdownView'
import { PageShell } from '../components/PageShell'
import { Sidebar } from '../components/Sidebar'
import type { DocResponse, TreeNode } from '../types'
import { useTheme } from '../useTheme'

function findNode(nodes: TreeNode[], target: string): TreeNode | null {
  for (const node of nodes) {
    if (node.path === target) return node
    if (node.children) {
      const found = findNode(node.children, target)
      if (found) return found
    }
  }
  return null
}

export function DocPage() {
  const params = useParams()
  const slug = params.slug ?? ''
  const currentPath = params['*'] ?? ''
  const navigate = useNavigate()
  const { canEdit } = useAuth()
  const dark = useTheme()

  const [tree, setTree] = useState<TreeNode[]>([])
  const [doc, setDoc] = useState<DocResponse | null>(null)
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState('')
  const [editing, setEditing] = useState(false)
  const [draft, setDraft] = useState('')
  const [editTitle, setEditTitle] = useState('')
  const [editPublic, setEditPublic] = useState(true)
  const [saving, setSaving] = useState(false)
  const [showHistory, setShowHistory] = useState(false)
  const [catalog, setCatalog] = useState<{ id: string; text: string }[]>([])

  const isDoc = currentPath.toLowerCase().endsWith('.md')

  const loadTree = useCallback(async () => {
    try {
      const data = await api.get<{ tree: TreeNode[] }>(`/sources/${slug}/tree`)
      setTree(data.tree ?? [])
    } catch {
      setTree([])
    }
  }, [slug])

  const loadDoc = useCallback(async () => {
    if (!isDoc) {
      setDoc(null)
      setLoading(false)
      return
    }
    setLoading(true)
    setError('')
    try {
      const data = await api.get<DocResponse>(
        `/sources/${slug}/doc?path=${encodeURIComponent(currentPath)}`,
      )
      setDoc(data)
      setDraft(data.content)
      setEditTitle(data.meta.title)
      setEditPublic(data.meta.public)
    } catch (err) {
      setDoc(null)
      setError(err instanceof ApiError ? err.message : '加载文档失败')
    } finally {
      setLoading(false)
    }
  }, [slug, currentPath, isDoc])

  useEffect(() => {
    void loadTree()
  }, [loadTree])

  useEffect(() => {
    setEditing(false)
    setShowHistory(false)
    void loadDoc()
  }, [loadDoc])

  const save = useCallback(async () => {
    if (!doc || saving) return
    setSaving(true)
    try {
      await api.put(`/sources/${slug}/doc`, {
        path: doc.path,
        content: draft,
        sha: doc.sha,
        meta: { title: editTitle, public: editPublic },
      })
      setEditing(false)
      await loadDoc()
      await loadTree()
    } catch (err) {
      window.alert(err instanceof ApiError ? err.message : '保存失败')
    } finally {
      setSaving(false)
    }
  }, [doc, draft, editTitle, editPublic, saving, slug, loadDoc, loadTree])

  useEffect(() => {
    if (!editing) return
    const handler = (event: KeyboardEvent) => {
      if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === 's') {
        event.preventDefault()
        void save()
      }
    }
    window.addEventListener('keydown', handler)
    return () => window.removeEventListener('keydown', handler)
  }, [editing, save])

  useEffect(() => {
    if (editing || !doc) {
      setCatalog([])
      return
    }
    const timer = window.setTimeout(() => {
      const headings = Array.from(document.querySelectorAll<HTMLElement>('.article-content h3'))
      headings.forEach((heading, index) => {
        heading.id = `catalog-${index}`
      })
      setCatalog(headings.map((heading, index) => ({ id: `catalog-${index}`, text: heading.textContent ?? '' })))
    }, 50)
    return () => window.clearTimeout(timer)
  }, [doc, editing])

  const createDoc = async () => {
    const dir = isDoc
      ? currentPath.slice(0, currentPath.lastIndexOf('/') + 1)
      : currentPath
        ? `${currentPath}/`
        : ''
    const name = window.prompt('新建文档路径', `${dir}新文档.md`)
    if (!name) return
    try {
      await api.post(`/sources/${slug}/doc`, { path: name })
      await loadTree()
      navigate(`/docs/${slug}/${name}`)
    } catch (err) {
      window.alert(err instanceof ApiError ? err.message : '创建失败')
    }
  }

  const createDir = async () => {
    const base = isDoc ? currentPath.slice(0, currentPath.lastIndexOf('/') + 1) : currentPath ? `${currentPath}/` : ''
    const name = window.prompt('新建目录名', `${base}新目录`)
    if (!name) return
    const target = `${name.replace(/\/$/, '')}/README.md`
    try {
      await api.post(`/sources/${slug}/doc`, { path: target, title: name })
      await loadTree()
      navigate(`/docs/${slug}/${target}`)
    } catch (err) {
      window.alert(err instanceof ApiError ? err.message : '创建失败')
    }
  }

  const remove = async () => {
    if (!doc) return
    if (!window.confirm(`确定删除 ${doc.path}？`)) return
    try {
      await api.del(`/sources/${slug}/doc?path=${encodeURIComponent(doc.path)}&sha=${encodeURIComponent(doc.sha)}`)
      await loadTree()
      navigate(`/docs/${slug}`)
    } catch (err) {
      window.alert(err instanceof ApiError ? err.message : '删除失败')
    }
  }

  const move = async () => {
    if (!doc) return
    const target = window.prompt('移动到新路径', doc.path)
    if (!target || target === doc.path) return
    try {
      await api.post(`/sources/${slug}/move`, { from: doc.path, to: target })
      await loadTree()
      navigate(`/docs/${slug}/${target}`)
    } catch (err) {
      window.alert(err instanceof ApiError ? err.message : '移动失败')
    }
  }

  const importFiles = async (files: File[]) => {
    const category = isDoc ? currentPath.slice(0, currentPath.lastIndexOf('/') + 1) : currentPath
    try {
      const payload = {
        category,
        files: await Promise.all(
          files.map(async (file) => ({ name: file.name, content: await file.text() })),
        ),
      }
      await api.post(`/sources/${slug}/import`, payload)
      await loadTree()
      window.alert(`已导入 ${files.length} 个文件`)
    } catch (err) {
      window.alert(err instanceof ApiError ? err.message : '导入失败')
    }
  }

  const reorder = async (dir: string, names: string[]) => {
    try {
      await api.put(`/sources/${slug}/meta`, { path: dir, order: names })
      await loadTree()
    } catch (err) {
      window.alert(err instanceof ApiError ? err.message : '排序失败')
    }
  }

  const segments = currentPath.split('/').filter(Boolean)
  const breadcrumb = (
    <>
      <a onClick={() => navigate('/')}>首页</a>
      {slug && (
        <>
          {' / '}
          <a onClick={() => navigate(`/docs/${slug}`)}>{slug}</a>
        </>
      )}
      {segments.map((segment, index) => {
        const target = segments.slice(0, index + 1).join('/')
        return (
          <span key={target}>
            {' / '}
            <a onClick={() => navigate(`/docs/${slug}/${target}`)}>{decodeURIComponent(segment)}</a>
          </span>
        )
      })}
    </>
  )

  const sidebar = (
    <Sidebar
      slug={slug}
      tree={tree}
      currentPath={currentPath}
      canEdit={canEdit}
      onReorder={reorder}
      onCreateDir={createDir}
    />
  )

  const directory = !isDoc ? findNode(tree, currentPath) : null
  const dirChildren = directory?.children ?? (currentPath === '' ? tree : [])

  const title = doc?.meta.title || directory?.name || ''

  return (
    <PageShell breadcrumb={breadcrumb} sidebar={sidebar} onDropFiles={canEdit ? importFiles : undefined}>
      {loading && <p>加载中…</p>}
      {!loading && error && (
        <div className="article-content">
          <h3>未找到内容</h3>
          <p>{error}</p>
        </div>
      )}

      {!loading && !error && isDoc && doc && (
        <>
          <h1 className="article-title">{title}</h1>

          {canEdit && !editing && (
            <div className="doc-actions">
              <button className="btn btn-primary" onClick={() => setEditing(true)}>
                <i className="fas fa-pen" /> 编辑
              </button>
              <button className="btn btn-default" onClick={() => setShowHistory((value) => !value)}>
                <i className="fas fa-history" /> 改动记录
              </button>
              <button className="btn btn-warning" onClick={() => void move()}>
                移动
              </button>
              <button className="btn btn-danger" onClick={() => void remove()}>
                删除
              </button>
              {!doc.meta.public && <span className="badge-private">仅登录可见</span>}
            </div>
          )}

          {editing ? (
            <div className="article-editor open">
              <div className="editor-toolbar">
                <input
                  className="form-control"
                  value={editTitle}
                  onChange={(event) => setEditTitle(event.target.value)}
                  placeholder="标题"
                />
                <label className="editor-check">
                  <input
                    type="checkbox"
                    checked={editPublic}
                    onChange={(event) => setEditPublic(event.target.checked)}
                  />
                  匿名可读
                </label>
                <span className="editor-spacer" />
                <button className="btn btn-primary" onClick={() => void save()} disabled={saving}>
                  {saving ? '保存中…' : '保存'}
                </button>
                <button
                  className="btn btn-warning"
                  onClick={() => {
                    setEditing(false)
                    setDraft(doc.content)
                  }}
                >
                  取消
                </button>
              </div>
              <Editor value={draft} onChange={setDraft} />
            </div>
          ) : (
            <MarkdownView content={doc.content} dark={dark} />
          )}

          {showHistory && <HistoryPanel slug={slug} path={doc.path} onClose={() => setShowHistory(false)} />}
        </>
      )}

      {!loading && !error && !isDoc && (
        <>
          <h1 className="article-title">{currentPath ? directory?.name || currentPath : '文档目录'}</h1>
          <div className="article-content">
            {dirChildren.length === 0 && <p>该目录下暂无文档</p>}
            {dirChildren.length > 0 && (
              <ol className="article-list">
                {dirChildren.map((node) => (
                  <li key={node.path}>
                    <a onClick={() => navigate(`/docs/${slug}/${node.path}`)}>
                      <i className={`fas ${node.type === 'dir' ? 'fa-folder' : 'fa-angle-right'}`} />{' '}
                      {node.title || node.name}
                      {node.private && <i className="fas fa-lock sidebar-lock" />}
                    </a>
                  </li>
                ))}
              </ol>
            )}
          </div>
          {canEdit && (
            <div className="admin-actions">
              <hr />
              <h3>管理操作</h3>
              <p>可以在此目录下新建文档，侧栏箭头用于调整同级顺序。</p>
              <p>
                <button className="btn btn-primary" onClick={() => void createDoc()}>
                  新建文档
                </button>
                <button className="btn btn-warning" onClick={() => void createDir()}>
                  新建子目录
                </button>
              </p>
            </div>
          )}
        </>
      )}

      {catalog.length > 0 && !editing && (
        <div className="article-catalog">
          <h3>目录</h3>
          <ul>
            {catalog.map((item) => (
              <li key={item.id} onClick={() => document.getElementById(item.id)?.scrollIntoView({ behavior: 'smooth' })}>
                {item.text}
              </li>
            ))}
          </ul>
        </div>
      )}
    </PageShell>
  )
}
