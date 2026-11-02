import { useEffect, useState } from 'react'
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

  const open = async (sha: string) => {
    try {
      const data = await api.get<{ commit: CommitDetail }>(`/sources/${slug}/commit/${sha}`)
      setDetail(data.commit)
    } catch {
      setDetail(null)
    }
  }

  return (
    <div className="history-panel">
      <div className="history-head">
        <strong>改动记录</strong>
        <a onClick={onClose}>
          <i className="fas fa-times" />
        </a>
      </div>

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
  )
}
