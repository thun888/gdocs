import { useCallback, useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { ApiError, api } from '../api'
import { useAuth } from '../auth'
import { PageShell } from '../components/PageShell'
import type { AdminGroup, AdminSource, AdminUser } from '../types'

type Tab = 'sources' | 'groups' | 'users' | 'settings'

const emptySource = {
  id: 0,
  slug: '',
  name: '',
  base_url: '',
  token: '',
  owner: '',
  repo: '',
  branch: 'main',
  root: '',
  sort: 0,
  enabled: true,
}

export function Admin() {
  const navigate = useNavigate()
  const { isAdmin, ready, login } = useAuth()
  const [tab, setTab] = useState<Tab>('sources')
  const [sources, setSources] = useState<AdminSource[]>([])
  const [groups, setGroups] = useState<AdminGroup[]>([])
  const [users, setUsers] = useState<AdminUser[]>([])
  const [form, setForm] = useState({ ...emptySource })
  const [settings, setSettings] = useState({ site_name: '', site_desc: '' })
  const [message, setMessage] = useState('')

  const load = useCallback(async () => {
    try {
      const [sourceData, groupData, userData, settingData] = await Promise.all([
        api.get<{ sources: AdminSource[] }>('/admin/sources'),
        api.get<{ groups: AdminGroup[] }>('/admin/groups'),
        api.get<{ users: AdminUser[] }>('/admin/users'),
        api.get<{ site_name: string; site_desc: string }>('/admin/settings'),
      ])
      setSources(sourceData.sources ?? [])
      setGroups(groupData.groups ?? [])
      setUsers(userData.users ?? [])
      setSettings({ site_name: settingData.site_name ?? '', site_desc: settingData.site_desc ?? '' })
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : '加载失败')
    }
  }, [])

  useEffect(() => {
    if (isAdmin) void load()
  }, [isAdmin, load])

  if (!ready) return <PageShell>加载中…</PageShell>
  if (!isAdmin) {
    return (
      <PageShell>
        <div className="article-content">
          <h3>需要管理员权限</h3>
          <p>当前账号没有管理权限，可以联系管理员把所在用户组设为 admin。</p>
          <button className="btn btn-primary" onClick={login}>
            重新登录
          </button>
        </div>
      </PageShell>
    )
  }

  const saveSource = async () => {
    const payload = { ...form, token: form.token }
    try {
      if (form.id) {
        await api.put(`/admin/sources/${form.id}`, payload)
      } else {
        await api.post('/admin/sources', payload)
      }
      setForm({ ...emptySource })
      await load()
      setMessage('已保存')
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : '保存失败')
    }
  }

  const removeSource = async (id: number) => {
    if (!window.confirm('确认删除该来源？')) return
    await api.del(`/admin/sources/${id}`)
    await load()
  }

  const syncSource = async (slug: string) => {
    setMessage('同步中…')
    try {
      await api.post('/admin/sync', { slug })
      setMessage('索引已重建')
    } catch (err) {
      setMessage(err instanceof ApiError ? err.message : '同步失败')
    }
  }

  const saveGroup = async (group: AdminGroup, role: string, ignored: boolean) => {
    await api.put(`/admin/groups/${group.id}`, { role, ignored })
    await load()
  }

  const saveSettings = async () => {
    await api.put('/admin/settings', settings)
    setMessage('设置已保存')
  }

  return (
    <PageShell
      breadcrumb={
        <>
          <a onClick={() => navigate('/')}>首页</a> / <span>管理</span>
        </>
      }
    >
      <h1 className="article-title">管理后台</h1>
      <div className="admin-page">
        <div className="admin-tabs" role="tablist">
          {(['sources', 'groups', 'users', 'settings'] as Tab[]).map((item) => (
            <button
              key={item}
              role="tab"
              aria-selected={tab === item}
              className={`admin-tab${tab === item ? ' active' : ''}`}
              onClick={() => setTab(item)}
            >
              {{ sources: '仓库来源', groups: '用户组', users: '用户', settings: '站点设置' }[item]}
            </button>
          ))}
        </div>

        {message && (
          <div className="admin-message" role="status">
            {message}
          </div>
        )}

        {tab === 'sources' && (
          <>
            <section className="admin-panel">
              <h2 className="admin-panel-title">仓库来源</h2>
              <div className="admin-table-wrap">
                <table className="admin-table">
                  <thead>
                    <tr>
                      <th>名称</th>
                      <th>slug</th>
                      <th>仓库</th>
                      <th>分支</th>
                      <th>状态</th>
                      <th>操作</th>
                    </tr>
                  </thead>
                  <tbody>
                    {sources.map((source) => (
                      <tr key={source.id}>
                        <td>{source.name}</td>
                        <td>{source.slug}</td>
                        <td>
                          {source.owner}/{source.repo}
                        </td>
                        <td>{source.branch}</td>
                        <td>{source.enabled ? '启用' : '停用'}</td>
                        <td>
                          <button className="btn btn-default btn-sm" onClick={() => setForm({ ...source, token: '' })}>
                            编辑
                          </button>
                          <button className="btn btn-default btn-sm" onClick={() => void syncSource(source.slug)}>
                            同步
                          </button>
                          <button className="btn btn-danger btn-sm" onClick={() => void removeSource(source.id)}>
                            删除
                          </button>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            </section>

            <section className="admin-panel">
              <h2 className="admin-panel-title">{form.id ? '编辑来源' : '新增来源'}</h2>
              <div className="admin-form">
                {(
                  [
                    ['slug', 'slug'],
                    ['name', '名称'],
                    ['base_url', '仓库地址'],
                    ['token', form.id ? '令牌（留空不修改）' : '访问令牌'],
                    ['owner', 'owner'],
                    ['repo', 'repo'],
                    ['branch', '分支'],
                    ['root', '子目录'],
                  ] as const
                ).map(([key, label]) => (
                  <label key={key}>
                    <span>{label}</span>
                    <input
                      className="form-control"
                      value={String(form[key] ?? '')}
                      onChange={(event) => setForm({ ...form, [key]: event.target.value })}
                    />
                  </label>
                ))}
                <label>
                  <span>排序</span>
                  <input
                    className="form-control"
                    type="number"
                    value={form.sort}
                    onChange={(event) => setForm({ ...form, sort: Number(event.target.value) })}
                  />
                </label>
                <label className="admin-inline">
                  <input
                    type="checkbox"
                    checked={form.enabled}
                    onChange={(event) => setForm({ ...form, enabled: event.target.checked })}
                  />
                  启用
                </label>
                <div className="admin-form-actions">
                  <button className="btn btn-primary" onClick={() => void saveSource()}>
                    保存
                  </button>
                  {form.id > 0 && (
                    <button className="btn btn-default" onClick={() => setForm({ ...emptySource })}>
                      取消编辑
                    </button>
                  )}
                </div>
              </div>
            </section>
          </>
        )}

        {tab === 'groups' && (
          <section className="admin-panel">
            <h2 className="admin-panel-title">用户组</h2>
            <p className="admin-hint">上游组来自登录声明，这里为每个组指定站点角色。未列出的组默认 viewer。</p>
            <div className="admin-table-wrap">
              <table className="admin-table">
                <thead>
                  <tr>
                    <th>组标识</th>
                    <th>类型</th>
                    <th>成员</th>
                    <th>角色</th>
                    <th>忽略</th>
                  </tr>
                </thead>
                <tbody>
                  {groups.map((group) => (
                    <tr key={group.id}>
                      <td>{group.code}</td>
                      <td>{group.type}</td>
                      <td>{group.members}</td>
                      <td>
                        <select
                          className="form-control"
                          value={group.role}
                          onChange={(event) => void saveGroup(group, event.target.value, group.ignored)}
                        >
                          <option value="viewer">viewer</option>
                          <option value="editor">editor</option>
                          <option value="admin">admin</option>
                        </select>
                      </td>
                      <td>
                        <input
                          type="checkbox"
                          checked={group.ignored}
                          onChange={(event) => void saveGroup(group, group.role, event.target.checked)}
                        />
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        )}

        {tab === 'users' && (
          <section className="admin-panel">
            <h2 className="admin-panel-title">用户</h2>
            <div className="admin-table-wrap">
              <table className="admin-table">
                <thead>
                  <tr>
                    <th>姓名</th>
                    <th>学号/工号</th>
                    <th>角色</th>
                    <th>用户组</th>
                    <th>最近登录</th>
                  </tr>
                </thead>
                <tbody>
                  {users.map((user) => (
                    <tr key={user.id}>
                      <td>{user.display_name}</td>
                      <td>{user.subject}</td>
                      <td>{user.role}</td>
                      <td>{user.groups.join('，')}</td>
                      <td>{user.last_login_at ? new Date(user.last_login_at).toLocaleString('zh-CN') : '从未'}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </section>
        )}

        {tab === 'settings' && (
          <section className="admin-panel">
            <h2 className="admin-panel-title">站点设置</h2>
            <div className="admin-form admin-form-narrow">
              <label>
                <span>站点名称</span>
                <input
                  className="form-control"
                  value={settings.site_name}
                  onChange={(event) => setSettings({ ...settings, site_name: event.target.value })}
                />
              </label>
              <label>
                <span>站点介绍</span>
                <input
                  className="form-control"
                  value={settings.site_desc}
                  onChange={(event) => setSettings({ ...settings, site_desc: event.target.value })}
                />
              </label>
              <div className="admin-form-actions">
                <button className="btn btn-primary" onClick={() => void saveSettings()}>
                  保存设置
                </button>
              </div>
            </div>
          </section>
        )}
      </div>
    </PageShell>
  )
}
