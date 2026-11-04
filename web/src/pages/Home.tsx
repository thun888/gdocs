import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { api } from '../api'
import { PageShell } from '../components/PageShell'
import type { Commit, Source } from '../types'

export function Home() {
  const navigate = useNavigate()
  const [sources, setSources] = useState<Source[]>([])
  const [recent, setRecent] = useState<Record<string, Commit[]>>({})
  const [loading, setLoading] = useState(true)

  useEffect(() => {
    api
      .get<{ sources: Source[] }>('/sources')
      .then(async (data) => {
        const list = data.sources ?? []
        setSources(list)
        const results: Record<string, Commit[]> = {}
        await Promise.all(
          list.map(async (source) => {
            try {
              const history = await api.get<{ commits: Commit[] }>(`/sources/${source.slug}/recent?limit=5`)
              results[source.slug] = history.commits ?? []
            } catch {
              results[source.slug] = []
            }
          }),
        )
        setRecent(results)
      })
      .catch(() => setSources([]))
      .finally(() => setLoading(false))
  }, [])

  return (
    <PageShell>
      <h1 className="article-title">文档</h1>
      <div className="article-content">
        {loading && <p>加载中…</p>}
        {!loading && sources.length === 0 && <p>暂无可访问的文档来源。</p>}

        <div className="source-grid">
          {sources.map((source) => (
            <div
              key={source.slug}
              className="source-card"
              role="link"
              tabIndex={0}
              onClick={() => navigate(`/docs/${source.slug}`)}
              onKeyDown={(event) => {
                if (event.key !== 'Enter' && event.key !== ' ') return
                event.preventDefault()
                navigate(`/docs/${source.slug}`)
              }}
            >
              <div className="source-name">{source.name}</div>
              <div className="source-slug">{source.slug}</div>
              {(recent[source.slug] ?? []).slice(0, 3).map((commit) => (
                <div key={commit.sha} className="source-commit">
                  <span>{commit.message}</span>
                  <em>{new Date(commit.date).toLocaleDateString('zh-CN')}</em>
                </div>
              ))}
            </div>
          ))}
        </div>
      </div>
    </PageShell>
  )
}
