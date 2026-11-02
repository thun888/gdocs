import { useNavigate } from 'react-router-dom'
import type { TreeNode } from '../types'

interface Props {
  slug: string
  tree: TreeNode[]
  currentPath: string
  canEdit: boolean
  onReorder: (dir: string, names: string[]) => void
  onCreateDir: () => void
}

export function Sidebar({ slug, tree, currentPath, canEdit, onReorder, onCreateDir }: Props) {
  const navigate = useNavigate()
  const go = (path: string) => navigate(`/docs/${slug}/${path}`)

  const move = (nodes: TreeNode[], index: number, dir: string, delta: number) => {
    const target = index + delta
    if (target < 0 || target >= nodes.length) return
    const next = nodes.slice()
    ;[next[index], next[target]] = [next[target], next[index]]
    onReorder(dir, next.map((node) => node.name))
  }

  const renderNodes = (nodes: TreeNode[], dir: string) => (
    <ul>
      {nodes.map((node, index) => {
        if (node.type === 'dir') {
          const active = currentPath === node.path || currentPath.startsWith(`${node.path}/`)
          return (
            <li key={node.path} className={active ? 'active' : undefined}>
              <a onClick={() => go(node.path)}>{node.name}</a>
              {renderNodes(node.children ?? [], node.path)}
            </li>
          )
        }
        const active = currentPath === node.path
        return (
          <li key={node.path} className={active ? 'active' : undefined}>
            <a className={`article-link${active ? ' active' : ''}`} onClick={() => go(node.path)}>
              {node.title || node.name}
              {node.private && <i className="fas fa-lock sidebar-lock" title="仅登录可见" />}
            </a>
            {canEdit && (
              <span className="reorder">
                <button onClick={() => move(nodes, index, dir, -1)} title="上移">
                  <i className="fas fa-angle-up" />
                </button>
                <button onClick={() => move(nodes, index, dir, 1)} title="下移">
                  <i className="fas fa-angle-down" />
                </button>
              </span>
            )}
          </li>
        )
      })}
    </ul>
  )

  return (
    <div className="categories">
      <ul>
        <li className="category-title">目录</li>
        {renderNodes(tree, '')}
      </ul>
      {canEdit && (
        <div className="admin-action">
          <button className="btn btn-block btn-primary" onClick={onCreateDir}>
            新建文档
          </button>
        </div>
      )}
    </div>
  )
}
