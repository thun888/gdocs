# ZDocs Go 复刻实施计划

> 原始需求：在当前工作空间写一个 Go 程序，前端用 `G:\Project\ZDocs` 的样式，用现代技术栈复刻，原生支持无刷新。技术栈：前端 React，后端 Gin，数据库 PostgreSQL。功能：① 以 OAuth 为唯一用户认证源，支持上游权限组管理；② 以 Forgejo（Gitea）为文件存储服务，可配置多个仓库来源，文档目录按真实文件目录显示；③ 利用 Git 服务器接口实现改动显示；④ 图片上传自动保存到 S3 兼容存储；⑤ 支持 Mermaid 与语法高亮。
>
> 补充约定：文档可见性由 front-matter 与全局默认共同决定，文档内优先；目录排序写入仓库内的独立文件；提交身份取当前用户姓名与邮箱；全局用户组限制可选开启。

## 1. 目标与边界

**目标**

- 单二进制部署的文档站，正文只存在于 Git 仓库，站点自身不存正文。
- 视觉与交互复刻 ZDocs：左侧目录树、顶栏面包屑、正文区、明暗主题、Cherry 编辑器、拖拽排序、拖文件导入。
- 全部交互走 API，页面不整页刷新；URL 可分享、可刷新还原。
- 权限来自上游 OIDC 组声明，站点不维护密码。

**非目标**

- 不提供本地账号、注册、改密。
- 不做多租户隔离、评论、独立全文检索引擎。
- 不做分支/PR 评审流（直接提交目标分支，后续可加保护分支）。
- 不兼容 `?id=1` 形式的旧文章 URL。

## 2. 总体架构

```
                 ┌──────────────────────────────┐
  Browser        │  React + Router + Query      │
  ────────────►  │  Cherry Markdown + ZDocs CSS │
                 └───────────────┬──────────────┘
                                 │ JSON /api/v1
                 ┌───────────────▼──────────────┐
                 │  Gin                         │
                 │  ├ auth    OIDC + session    │
                 │  ├ source  Forgejo client    │
                 │  ├ doc     index / meta/path │
                 │  ├ storage S3 uploader       │
                 │  └ store   PostgreSQL        │
                 └───┬──────────┬───────────┬───┘
                     │          │           │
               ┌─────▼───┐ ┌────▼────┐ ┌────▼────┐
               │ Forgejo │ │  GZHU   │ │   S3    │
               │ 多仓库   │ │   SSO   │ │ 兼容    │
               └─────────┘ └─────────┘ └─────────┘
```

职责边界：**Git 存正文与历史，S3 存图片，PostgreSQL 存用户、组、会话、来源、文档索引**。正文不落库。

## 3. 技术选型

| 层 | 选择 | 说明 |
|---|---|---|
| HTTP | Gin v1 | 需求指定 |
| 配置 | `gopkg.in/yaml.v3` + `${ENV}` 展开 | 密钥走环境变量 |
| DB | PostgreSQL 16 + GORM（`pgx` 驱动） | 连接池、`goose` 管理迁移 |
| 认证 | `coreos/go-oidc/v3` + `golang.org/x/oauth2` | 授权码 + PKCE(S256) + state/nonce |
| 会话 | 服务端会话表 + 签名 Cookie（只放会话 ID） | 可即时踢人，令牌不出服务端 |
| Git | Forgejo `/api/v1`（Gitea 兼容） | 不 clone，全走 API |
| 索引 | 单次下载仓库 archive 解析 | 避免逐文件 GET |
| 对象存储 | `minio-go/v7` | 兼容 MinIO / R2 / OSS |
| Markdown | cherry-markdown + DOMPurify | 与 PHP 版渲染同源 |
| 图表 | mermaid 11 | 前端渲染，跟随主题 |
| 高亮 | highlight.js 11 + atom-one 双主题 | 复用 ZDocs 现有 css |
| 前端 | React 19 + TypeScript + Vite + React Router | 无刷新；请求用轻量 fetch 封装 |

前端产物 `web/dist` 用 `go:embed` 打进二进制。

## 4. 配置

```yaml
server:
  addr: ":8080"
  base_url: "https://docs.example.com"
  session_key: "${ZDOCS_SESSION_KEY}"    # ≥32 字节
  public_read: true                      # 全局默认可见性，文档 front-matter 可覆盖

database:
  dsn: "postgres://zdocs:${PG_PASSWORD}@127.0.0.1:5432/zdocs?sslmode=disable"
  max_open: 20
  max_idle: 5
  auto_migrate: false

oauth:
  issuer: "https://gzhu-sso.school.hzchu.top"
  client_id: "zdocs"
  client_secret: "${OAUTH_CLIENT_SECRET}"
  redirect_url: "https://docs.example.com/api/v1/auth/callback"
  scopes: ["openid", "profile", "email", "groups"]
  groups_claim: "groups"
  bootstrap_admin: ["20230001"]          # 学号/教工号，豁免组限制

access:
  allowed_groups: []                     # 空为不限制；命中任一即放行，支持 * 结尾通配
  deny_message: "当前账号不在 ZDocs 的访问范围内"

git:
  bot_name: "ZDocs"
  bot_email: "zdocs@example.com"         # 用户邮箱缺失时的兜底提交身份

index:
  ttl_minutes: 10
  sync_on_write: true
  max_archive_mb: 50

s3:
  endpoint: "https://s3.example.com"
  region: "us-east-1"
  bucket: "zdocs"
  access_key: "${S3_ACCESS_KEY}"
  secret_key: "${S3_SECRET_KEY}"
  path_prefix: "docs/"
  public_base: "https://cdn.example.com/zdocs"
  max_size_mb: 10
  allow_types: ["image/png", "image/jpeg", "image/gif", "image/webp", "image/svg+xml"]

sources:
  - slug: "handbook"
    name: "员工手册"
    base_url: "https://git.example.com"
    token: "${FORGEJO_TOKEN_HANDBOOK}"
    owner: "docs"
    repo: "handbook"
    branch: "main"
    root: ""
    sort: 10
```

规则：`-c` 指定配置，默认 `./config.yaml`；`${VAR}` 加载时展开，缺失即启动失败；`sources` 由后台写 DB（DB 优先，YAML 作种子）。

## 5. 认证与权限

上游为广州大学统一身份 `https://gzhu-sso.school.hzchu.top`（`gzhu-sso-relay`），标准 OIDC 授权码流程，ID Token RS256，access token 1 小时，refresh token 30 天且**每次刷新轮转、旧值立即失效**。端点：`/oidc/authorize`、`/oidc/token`、`/oidc/userinfo`、`/oidc/jwks`、`/oidc/revoke`；无 `end_session`、无 introspection。

**接入前置**：relay 后台新建应用，回调填 `{base_url}/api/v1/auth/callback`，**必须勾选 `groups` scope**（默认只开 `openid profile email`），否则 ID Token 不含组，所有人落成 viewer。

```bash
./bin/relayctl -config config.yaml client create \
  -id zdocs -name "ZDocs" \
  -redirect "https://docs.example.com/api/v1/auth/callback" \
  -scopes openid,profile,email,groups
```

**登录**：`/auth/login` 生成 state/nonce/PKCE(S256) 后 302 到 authorize（首次申请 `groups` 多一次授权确认页）；回调校验 `state`、换 token、验 id_token（`iss`/`aud`/`exp`/`nonce`/签名，JWKS 缓存 1 小时，未知 `kid` 立即刷新），以 `sub` upsert 用户，写会话后回跳。

**claim 映射**

| claim | 用途 |
|---|---|
| `sub` | 用户主键，学号/教工号，稳定不变 |
| `preferred_username` | 登录名，与 `sub` 相同 |
| `name` | 显示名，同时作为提交 author name |
| `email`、`email_verified` | 联系邮箱，同时作为提交 author email |
| `belong_class` | 班级，仅展示 |
| `groups` | 权限组 code 数组，含祖先组；无组时整个 claim 缺失 |

relay 不返回头像，头像用姓名首字生成。

**组 code 结构**：`user_type:学生` → `unit:学生/计算机学院` → `major:学生/计算机学院/计算机科学与技术`。`groups` 已含祖先，给最细一级赋权即覆盖整条链；赋权按 code 精确匹配，不做通配。

**角色重算**：登录成功与每次令牌刷新时，用最新 `groups` 整体覆盖 `user_groups`，有效角色取所属组最高值。上游移出组后最迟 1 小时降权；用户被禁用时刷新返回 `invalid_grant`，会话立即作废。

**全局组限制（可选）**：`access.allowed_groups` 非空时，登录回调与每次刷新都要校验 `groups` 与名单有交集（支持 `unit:学生/计算机学院*` 这样以 `*` 结尾的前缀），不命中则拒绝登录并返回 403 页面，不建会话。名单里的人被移出组后，最迟一个刷新周期被踢出。`bootstrap_admin` 名单豁免，防止误配置把自己锁在门外。relay 侧的「限定用户组」是另一道门，与本配置互相独立，两者都配就是交集。

**会话**：`sessions` 表存会话 ID、uid、加密的 access/refresh token、过期时间；Cookie 只放签名后的会话 ID（HttpOnly、SameSite=Lax、7 天）。access token 到期前 5 分钟用 refresh token 换新并重算角色；刷新失败即清会话、返回 401，前端转未登录态。

**登出**：删会话并调 `/oidc/revoke` 撤销 refresh token。relay 无 `end_session`，门户侧仍保持登录，属已知限制。

**CSRF**：写请求要求 `X-CSRF-Token` 双提交与 `Content-Type: application/json`；登出用 POST。

## 6. 内容模型与可见性

**目录即分类**：仓库树直接映射侧边栏，支持任意层级。目录下 `README.md` 作为该目录首页。

**文档格式**：Markdown + YAML front-matter。

```markdown
---
title: 快速开始
order: 10
public: false
draft: false
---

正文
```

| 字段 | 语义 |
|---|---|
| `title` | 缺省取文件名 |
| `order` | 同级排序的兜底值，缺省按文件名 |
| `public` | 覆盖全局 `server.public_read`；未写则继承 |
| `draft` | `true` 时仅登录用户可见，优先级高于 `public` |

**可见性判定（文档内优先）**

```
draft: true            → 仅登录可见（无论 public）
public 显式存在        → 用它
public 未定义          → server.public_read
```

匿名请求命中的文档为不可见时返回 **404 而非 403**，避免泄露路径存在性。目录列表按同一规则过滤：文件级过滤后为空的目录整枝隐藏；目录自身的 `README.md` 若是私有的，目录名仍可显示（其子项可见时），但不渲染首页内容。

**排序文件**：每个目录可选 `_meta.yaml`，是仓库里唯一由站点写入的额外文件。

```yaml
order: ["intro.md", "guide/", "faq.md"]
```

未列出的条目按 front-matter `order` → 文件名递增排后。拖动排序只改这一个文件，不重写文档；移动/改名时同步修正上一级与下一级 `_meta.yaml`。

**写入与提交身份**：`PUT contents` 携带读取时的 blob `sha` 做乐观锁，冲突返回 409 与最新 sha。`author` 与 `committer` 都取当前登录用户的 `name` + `email`（来自 claim），邮箱缺失时回落到 `git.bot_name` / `git.bot_email`。提交信息默认 `docs: update {path}`，保存弹窗可改。非注册邮箱的提交在 Forgejo 里不会关联到账号，属预期行为。

**路径安全**：拒绝绝对路径、`..`、空段、反斜杠、NUL、超长段；与 `source.root` 拼接后二次校验仍在其内。

**多源**：`slug` 作 URL 前缀 `/docs/{slug}/{path}`；各源独立 token、索引与缓存；单源故障返回 502，不影响其他源。

## 7. 数据模型

| 表 | 字段 | 说明 |
|---|---|---|
| `users` | id, subject, username, display_name, email, class, role, created_at, last_login_at | `subject` 唯一，取 OIDC `sub`；`role` 为算出的有效角色 |
| `groups` | id, code, name, type, role, ignored, last_seen_at | `code` 唯一；`type` 为 `user_type`/`unit`/`major` |
| `user_groups` | user_id, group_id | 联合主键；登录/刷新时整体覆盖 |
| `sessions` | id, uid, access_token_enc, refresh_token_enc, expires_at, last_seen_at, ip, ua | 会话 ID 随机 32 字节，令牌 AES-GCM |
| `sources` | id, slug, name, base_url, token_enc, owner, repo, branch, root, sort, enabled | `token_enc` 用 AES-GCM |
| `doc_meta` | source_id, path, blob_sha, title, `order`, public, draft, updated_at | 索引缓存，主键 `(source_id, path)` |
| `settings` | key, value | 站点名、描述、默认源 |
| `audit_logs` | id, uid, action, target, detail, ip, created_at | 管理操作留痕（M6） |

`doc_meta` 是唯一允许冗余正文元信息的地方，只存判定所需的字段，随时可由仓库重建。新组默认 `viewer`，管理员可忽略（`ignored=true`）使其不参与角色计算。

**索引构建**：`GET /repos/{owner}/{repo}/archive/{branch}.tar.gz` 一次拉全量，流式遍历，只解析 `*.md` 的 front-matter 与各目录 `_meta.yaml`，写 `doc_meta`。触发时机：启动、写操作成功后（`sync_on_write`）、TTL 到期、后台手动 `/admin/sync`。同步用 singleflight 去重；失败保留旧索引并标记 stale，读路径不阻塞。

## 8. API 设计

统一前缀 `/api/v1`，响应 `{code, message, data}`，错误走 HTTP 状态码。

**认证**

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/auth/login?redirect=` | 302 到 SSO |
| GET | `/auth/callback` | 校验、建会话、回跳 |
| POST | `/auth/logout` | 删会话 + 撤销 refresh token |
| GET | `/me` | 用户、角色、所属组、可写源 |

**读**

| 方法 | 路径 | 说明 |
|---|---|---|
| GET | `/sources` | 可见源列表 |
| GET | `/sources/:slug/tree?path=` | 已按可见性过滤的目录树，含排序结果 |
| GET | `/sources/:slug/doc?path=` | `{content, sha, meta, path}`；不可见返回 404 |
| GET | `/sources/:slug/history?path=&limit=` | 文件提交历史，沿用文档可见性 |
| GET | `/sources/:slug/commit/:sha` | 单次提交的文件级 patch |
| GET | `/sources/:slug/compare?base=&head=` | 区间 diff |
| GET | `/sources/:slug/recent?limit=` | 源级最近改动，只含可见文档 |

**写（editor）**

| 方法 | 路径 | 说明 |
|---|---|---|
| PUT | `/sources/:slug/doc` | `{path, content, sha, message}`，409 冲突 |
| POST | `/sources/:slug/doc` | 新建 `{path, content, message}` |
| DELETE | `/sources/:slug/doc?path=&sha=` | 删除 |
| POST | `/sources/:slug/move` | `{from, to, sha}` 移动/改名，同步修正排序文件 |
| PUT | `/sources/:slug/meta` | 写 `_meta.yaml` |
| POST | `/sources/:slug/import` | 批量导入多文件 |
| POST | `/admin/sync` | 强制重建某源索引 |

**上传（editor）**

| 方法 | 路径 | 说明 |
|---|---|---|
| POST | `/uploads` | multipart 直传 S3，返回 `{url, key, size, mime}` |
| POST | `/uploads/presign` | 大文件直传（可选） |

**管理（admin）**

| 方法 | 路径 | 说明 |
|---|---|---|
| GET/POST/PUT/DELETE | `/admin/sources[/:id]` | CRUD，保存前用 `GET /version` 探活 |
| GET | `/admin/users` | 用户列表，含所属组 |
| GET | `/admin/groups` | 组树（按 `type` 分层），含成员数与角色 |
| PUT | `/admin/groups/:id` | 赋角色、忽略组 |
| GET/PUT | `/admin/settings` | 站点设置 |

## 9. 前端

**路由**

| 路径 | 页面 |
|---|---|
| `/` | 首页：源列表 + 最近改动 |
| `/docs/:slug/*` | 阅读（目录或文件） |
| `/docs/:slug/*?edit=1` | 编辑态（编辑器覆盖正文区，不跳页） |
| `/history/:slug/*` | 文档历史 |
| `/diff/:slug/:sha` | 单次提交 diff |
| `/admin/*` | 管理后台 |

`/docs/...` 为真实 URL，刷新可还原；页内切换用 `router.push` + 组件状态，不整页刷新。

**样式复用**：原样拷贝 `G:\Project\ZDocs\public\css` 到 `web/src/assets/css`，保留 `materialize.css`。React 组件沿用原 DOM 类名与层级（`.main-container > .navbar / .categories / .main-articles`），使 `style.css` 不改一行即可生效；只把 jQuery 的 DOM 操作换成状态驱动。`webfonts`、`images`、`favicon.ico` 一并迁移。

**状态**：`AuthContext` 管用户与角色，数据用 `fetch` 封装按需拉取；树懒加载；草稿保存在编辑器内存，保存失败不退编辑态。

**编辑器**：`<CherryEditor value onChange />`，`useEffect` 里 `new Cherry({id, value, ...})`，`afterChange` 回调受控更新，卸载时 `destroy()`。关掉内置上传，改接 `/uploads`。Ctrl/Cmd+S 触发 PUT，成功后更新 sha 并清草稿。可见性开关直接写 front-matter 的 `public`，保存时由服务端归一化。

**阅读渲染**：Cherry `engine.makeHtml` → 后处理 Mermaid → DOMPurify → `dangerouslySetInnerHTML`。链路与 PHP 版一致，同时修掉 `htmlWhiteList: '.*'` 的存储型 XSS。

**Mermaid**：```mermaid 代码块提取源码 → `mermaid.render(id, code)` → 替换节点。`securityLevel: 'strict'`、`startOnLoad: false`，主题跟随明暗，切主题重新渲染；失败保留源码并提示。编辑器内由 Cherry 插件实时预览，共用同一份主题配置。

**语法高亮**：阅读态 HTML 插入后统一 `hljs.highlightAll()`，只处理未高亮的 `pre code`；明暗切换换 `atom-one-light` / `atom-one-dark` 并重新着色。代码块补语言标签与复制按钮，超高折叠。

**保留的交互**：编辑态改标题与可见性、侧栏分组排序、拖 `.md` 进正文区导入、面包屑、明暗主题（跟随系统 + 21:00–6:00 自动暗色，首屏前内联 script 设 class 消除白闪）、h3 自动生成右栏目录并滚动高亮。

**去掉的**：jQuery、Materialize JS、SweetAlert2、100ms 轮询高亮。

## 10. 关键流程

**登录**：点登录 → `/auth/login?redirect=` → SSO（首次多一步 groups 确认）→ `/auth/callback` 校验、组限制检查、建会话 → 回原路径。`/me` 决定侧栏是否显示新建/编辑/管理入口。

**会话续期**：access token 剩 5 分钟时用 refresh token 换新（轮转写回），同时 userinfo 重算角色与组限制；401 或 `invalid_grant` 清会话。

**浏览**：进 `/docs/:slug/a/b` → 拉 tree（服务端已按可见性过滤）→ 侧栏渲染 → 命中文档拉 doc → Cherry 渲染 → Mermaid 替换 + 高亮。

**编辑保存**：进编辑态 → 拉原文与 sha → 编辑 → Ctrl+S → `PUT doc`（带 author/committer）→ 200 更新 sha、触发索引同步、退出编辑；409 弹「远端已变更」并用 diff2html 并排展示本地与远端。

**排序**：点侧栏上下箭头 → 前端生成新顺序 → `PUT meta` → 写 `_meta.yaml`（同样带用户身份提交）→ 失效该目录索引。

**改动显示**：文章页「历史」抽屉列 commits；点条目进 `/diff/:slug/:sha`，用 `git/commits/{sha}` 的 `files[].patch` 渲染；首页汇总各源 `recent`。

**图片上传**：Cherry 粘贴/拖入 → `POST /uploads` → 校验 MIME 与大小 → `PutObject` 到 `{prefix}{yyyy}/{mm}/{sha256前16}{ext}` → 回 `public_base/key` → 插入 `![](url)`。内容寻址天然去重，仓库只留引用 URL。

## 11. 目录结构

```
zdocs-go/
├── cmd/zdocs/main.go
├── cmd/migrate-php/main.go
├── internal/
│   ├── config/                    # 加载、${ENV} 展开、校验
│   ├── server/                    # gin 路由、中间件、静态托管
│   ├── auth/                      # oidc、session、rbac、csrf、组限制
│   ├── source/                    # Source 接口 + forgejo 实现
│   ├── doc/                       # 索引、front-matter、可见性、排序、路径
│   ├── storage/                   # s3 上传
│   ├── store/                     # gorm 模型与仓储
│   └── web/                       # go:embed dist
├── web/
│   ├── src/{pages,components,hooks,api,assets}
│   └── vite.config.ts
├── migrations/                    # goose SQL 迁移
├── config.example.yaml
├── go.mod
└── PLAN.md
```

## 12. 构建组织

**依赖方向**（只允许从左到右，禁止反向引用）

```
config → store → source ─┐
              → auth    ─┼→ handler → server
              → doc     ─┤
              → storage ─┘
```

| 包 | 职责 | 不做什么 |
|---|---|---|
| `config` | 读 YAML、展开 `${ENV}`、校验必填 | 不建连接、不读 DB |
| `store` | GORM 模型、迁移、仓储方法 | 不含业务判定 |
| `source` | Forgejo 客户端，返回领域对象与 `ErrNotFound`/`ErrConflict`/`ErrUpstream` | 不碰 HTTP 层 |
| `doc` | 路径清洗、front-matter、`_meta.yaml`、索引同步、可见性与排序计算 | 不直接发请求（经 `source`） |
| `auth` | OIDC 客户端、会话、角色解析、组限制、CSRF | 不知道文档概念 |
| `storage` | S3 上传、key 生成、MIME 校验 | 不生成 HTML |
| `server` | 路由、中间件、错误映射、embed 静态 | 不写 SQL |

**接口先行**：先定 `source.Source`（`Tree/Read/Write/Create/Delete/Move/History/Commit/Compare/Archive`）与 `doc.Index`，再写实现。M2 之前用 `source/fake` 走 fixture，前后端可各自推进。

**关键纯函数**（好测、先写表驱动用例）：`doc.ResolveVisibility(frontmatter, globalDefault)`、`auth.ResolveRole(groups)`、`auth.Allowed(groups, patterns)`、`doc.SafeJoin(root, path)`。

**构建方式**：`make dev` 起 vite dev（代理 `/api` 到 8080）与 `go run`；`make build` 先 `npm run build` 输出到 `internal/web/dist`，再 `go build -trimpath`；`make migrate` 跑 goose。生产单二进制，前端资源走 embed，静态与 API 同源，无 CORS。

**推进节奏**：每个里程碑结束跑一次 `go vet` 与 `make build`，按 `AGENTS.md` 提交一条语义化 commit；跨里程碑的重构不留到后面，接口一旦被前端消费就冻结签名。

## 13. 迁移

`cmd/migrate-php` 读旧库 `categories`/`articles`：分类层级 → 目录，`title` → front-matter，`content` → 正文，`order` → `_meta.yaml`，`author/time` 作为首个提交的 author。密码哈希丢弃（改 SSO）。分批提交，可回滚。

## 14. 里程碑

| 阶段 | 内容 | 验收 | 提交 |
|---|---|---|---|
| M0 | 骨架：配置、PG 连接与迁移、Gin 路由、embed 静态、日志 | 服务起得来，`/healthz` 连库通过 | `chore: scaffold gin server` |
| M1 | 认证：SSO 登录、会话、组同步与限制、赋权、`/me` | 登录登出闭环，改组与全局限制均生效 | `feat: oidc login with group roles` |
| M2 | 索引与只读：archive 解析、可见性、树、文档、Mermaid、高亮 | 私有文档匿名 404，目录树与真实目录一致 | `feat: browse repo tree` |
| M3 | 写入：编辑、新建、删除、移动、409、用户身份提交 | 提交作者为当前用户，冲突有提示 | `feat: edit docs via forgejo api` |
| M4 | 上传：S3 直传、编辑器接入 | 粘贴图片即得可访问 URL | `feat: upload images to s3` |
| M5 | 改动：历史、diff、最近改动 | 历史与 `git log` 一致 | `feat: show history and diffs` |
| M6 | 打磨：排序、导入、主题、TOC、后台 | 交互与 ZDocs 对齐 | `feat: polish admin and editor` |

## 15. 决策与风险

| 决策 | 备选 | 取舍 |
|---|---|---|
| Git 为唯一正文源 | 数据库存正文再同步 | 无双写；代价是每次读都过 API |
| 文档元信息进索引表 | 每次读都解析 front-matter | 列表页只需要一次 archive 同步，代价是多一张缓存表 |
| 可见性文档级覆盖全局 | 只按源/站点控制 | 满足单篇私密；代价是索引必须解析全部文档头 |
| 排序写 `_meta.yaml` | 每文件 front-matter order | 排序只产生 1 个提交 |
| 服务端会话 | Cookie 直接放 JWT | 令牌不出服务端、可即时踢人 |
| 组按 code 精确赋权 | 前缀/正则匹配 | relay 的 code 已含层级，精确匹配不易误授 |
| 提交身份用当前用户 | 统一机器人账号 | 历史可追溯到人；代价是 Forgejo 里多为未关联作者 |
| Mermaid 前端渲染 | 服务端渲染 SVG | 免 Node 依赖，可跟随主题 |

**风险**：relay 未勾选 `groups` scope 会导致全员 viewer；refresh token 轮转必须串行，多标签页并发刷新会拿到 `invalid_grant`，前端需单飞锁；relay 无 `end_session`，登出无法联动门户；archive 同步在大仓库上耗时，需限制 `max_archive_mb` 并保留旧索引；大仓库 `trees?recursive=true` 分页上限 1000，需循环取全；令牌需 contents 写权限；S3 公开读依赖 CDN 或桶策略；并发编辑靠 sha 乐观锁。

## 16. 部署前确认

1. relay 上的 zdocs 应用是否已勾选 `groups` scope。
2. Forgejo 令牌是否具备 contents 读写权限，仓库是否允许直接提交到目标分支。
3. S3 桶是否允许公开读，或改用签名 URL。
4. `server.public_read` 与 `access.allowed_groups` 的最终取值。

## 17. 实现状态

已完成：

- PG 迁移与连接池、配置加载、健康检查、embed 静态托管
- SSO 登录、会话续期、角色重算、全局组限制、CSRF
- 仓库 archive 索引、可见性过滤、目录树、文档读取
- 编辑、新建、删除、移动、元信息合并、提交身份、409 冲突
- 文章历史、单次提交 patch、diff 渲染
- S3 上传接口与编辑器接入
- 后台来源、用户组、用户、设置、索引重建
- React 前端：ZDocs 样式、Mermaid、代码高亮、右栏目录、明暗主题、拖入导入、排序

已验证：迁移在 PostgreSQL 16 上执行成功，`/healthz` 连库通过，`/api/v1/site`、`/api/v1/sources`、SPA 回退均返回预期结果，匿名与登录两种目录树过滤正确。

待在真实环境验证：Forgejo archive 与 contents 行为、SSO 回调与 groups 声明、S3 桶策略。

实现取舍：状态管理用轻量 fetch 封装而非 TanStack Query；diff 用自研行级渲染而非 diff2html；排序用侧栏箭头而非拖拽，均以稳定为先。
