import { useEffect, useState } from 'react'
import { createPortal } from 'react-dom'
import { api } from '../api'
import type { Commit, CommitDetail } from '../types'

function diffClass(line: string): string {
  if (line.startsWith('@@')) return 'hunk'
  if (line.startsWith('+++') || line.startsWith('---')) return 'meta'
  if (line.startsWith('+')) return 'add'
  if (line.startsWith('-')) return 'del'
  return 'ctx'
}

export function HistoryPanel({ slug, path, onClose }: { slug: string; path: string; onClose: () => void }) {
  const [commits, setCommits] = useState<Commit[]>([])
  const [detail, setDetail] = useState<CommitDetail | null>(null)
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    setLoading(true)
    api
      .get<{ commits: Commit[] }>(`/sources/${slug}/history?path=${encodeURIComponent(path)}`)
      .then((data) => setCommits(data.commits ?? []))
      .catch(() => setCommits([]))
      .finally(() => setLoading(false))
  }, [slug, path])

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') onClose()
    }
    window.addEventListener('keydown', onKey)
    const overflow = document.body.style.overflow
    document.body.style.overflow = 'hidden'
    return () => {
      window.removeEventListener('keydown', onKey)
      document.body.style.overflow = overflow
    }
  }, [onClose])

  const open = async (sha: string) => {
    try {
      const data = await api.get<{ commit: CommitDetail }>(`/sources/${slug}/commit/${sha}`)
      setDetail(data.commit)
    } catch {
      setDetail(null)
    }
  }

  return createPortal(
    <div className="modal-overlay" onClick={onClose}>
      <div
        className="modal"
        role="dialog"
        aria-modal="true"
        aria-label="改动记录"
        onClick={(event) => event.stopPropagation()}
      >
        <div className="modal-head">
          <strong>改动记录</strong>
          <button className="modal-close" onClick={onClose} aria-label="关闭">
            <i className="fas fa-times" />
          </button>
        </div>
        <div className="modal-body">
          {loading && <p>加载中…</p>}
          {!loading && commits.length === 0 && <p>暂无记录</p>}

          <ul className="history-list">
            {commits.map((commit) => (
              <li key={commit.sha} onClick={() => void open(commit.sha)}>
                <div className="history-message">{commit.message}</div>
                <div className="history-meta">
                  {commit.author} · {new Date(commit.date).toLocaleString('zh-CN')} · {commit.sha.slice(0, 7)}
                </div>
              </li>
            ))}
          </ul>

          {detail && (
            <div className="commit-detail">
              <div className="history-message">{detail.message}</div>
              {detail.files?.map((file) => (
                <div key={file.filename} className="diff-file">
                  <div className="diff-head">
                    {file.filename}
                    <span className="diff-stat">
                      <em>+{file.additions}</em> <del>-{file.deletions}</del>
                    </span>
                  </div>
                  <pre className="diff-block">
                    {(file.patch ?? '').split('\n').map((line, index) => (
                      <div key={index} className={`diff-line ${diffClass(line)}`}>
                        {line || ' '}
                      </div>
                    ))}
                  </pre>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>,
    document.body,
  )
}
