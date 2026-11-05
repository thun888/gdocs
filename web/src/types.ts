export interface SiteInfo {
  name: string
  description: string
  public_read: boolean
  header_show_avatar: boolean
  header_show_name: boolean
  header_show_logout: boolean
}

export interface User {
  id: number
  subject: string
  username: string
  display_name: string
  email: string
  class: string
  role: 'viewer' | 'editor' | 'admin'
  avatar: string
}

export interface Group {
  code: string
  name: string
  type: string
  role: string
}

export interface MeResponse {
  user: User | null
  groups: Group[]
  csrf?: string
  public_read: boolean
}

export interface Source {
  slug: string
  name: string
  sort: number
}

export interface TreeNode {
  name: string
  path: string
  type: 'dir' | 'doc'
  title?: string
  private?: boolean
  draft?: boolean
  children?: TreeNode[]
}

export interface DocMeta {
  title: string
  order: number
  public: boolean
  draft: boolean
}

export interface DocResponse {
  path: string
  sha: string
  content: string
  meta: DocMeta
}

export interface Commit {
  sha: string
  message: string
  author: string
  email: string
  date: string
  url: string
}

export interface AdminSource {
  id: number
  slug: string
  name: string
  base_url: string
  owner: string
  repo: string
  branch: string
  root: string
  sort: number
  enabled: boolean
}

export interface AdminGroup {
  id: number
  code: string
  name: string
  type: string
  role: string
  ignored: boolean
  members: number
}

export interface AdminUser {
  id: number
  subject: string
  username: string
  display_name: string
  email: string
  role: string
  groups: string[]
  last_login_at: string | null
}
