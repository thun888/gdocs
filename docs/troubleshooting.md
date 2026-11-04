# 故障排查

## 启动失败

### 加载配置失败

```text
加载配置失败 error=读取配置 config.yaml: ...
```

配置文件不存在或路径不对。用 `-c` 指定实际路径。

```text
环境变量未设置: GDOCS_SESSION_KEY
```

配置里的 `${VAR}` 没有对应环境变量。补齐环境变量，或直接写值。

```text
配置校验失败:
  - server.session_key 至少 32 字节
```

按提示逐项修正。常见问题：

- `session_key` 太短。
- `oauth.redirect_url` 不是绝对地址。
- `sources` 缺少 `name`、`base_url`、`token`、`owner` 或 `repo`。
- `sources.slug` 含大写字母、下划线或中文。

### 连接数据库失败

检查：

- PostgreSQL 是否启动。
- `database.dsn` 的账号、密码、库名、端口。
- 数据库是否允许来自服务所在机器的连接。
- `sslmode` 是否与服务端配置匹配。

本地可以先用 `psql` 验证同一个连接串。

### 数据库迁移失败

确认数据库账号有建表权限。迁移由二进制内嵌执行，不需要额外文件。

### 服务异常退出

```text
服务异常退出 error=listen tcp :8080: bind: address already in use
```

端口被占用。停止占用进程，或修改 `server.addr`。

## 登录问题

### 登录后提示不在访问范围内

对应配置 `access.allowed_groups`。检查 OIDC 返回的 `groups` 声明是否与配置的 code 完全匹配。支持以 `*` 结尾的前缀匹配。

### 所有用户都是 viewer

通常是没有拿到用户组：

- OIDC 应用没有授权 `groups` scope。
- `groups` 声明为空。
- 管理后台把相关组设为了“忽略”。

`oauth.groups_claim` 当前不参与解析，组声明固定读取 `groups`。

### 一直跳转登录或登录后立刻退出

- 确认 `server.base_url` 与浏览器实际访问的协议、域名一致。
- HTTPS 部署时 `base_url` 必须是 `https://`，否则 Cookie 行为可能不符合预期。
- 确认浏览器允许 Cookie。
- 确认服务器时间准确，OIDC 的 state 和令牌都依赖时间。

### CSRF 校验失败

非 GET 请求需要携带登录后由 `/api/v1/me` 返回的 `X-CSRF-Token`。浏览器前端会自动处理；直接调接口时需要先登录并带上该请求头。刷新页面或重新登录可以恢复。

### 401 未登录

会话已过期或被服务端清除。重新登录即可。

## 仓库与索引

### 目录树为空

按顺序检查：

1. 仓库是否有提交。空仓库没有 `main` 分支，归档接口返回 404。
2. `owner`、`repo`、`branch` 是否正确。
3. `root` 是否指向实际存在的子目录。
4. 文档是否以 `.md` 结尾。
5. `server.public_read` 是否为 `false`，且文档没有设置 `public: true`。匿名访问时私有文档不会出现在树里。

可以直接请求 Forgejo 验证：

```bash
curl -H "Authorization: token $TOKEN" \
  https://git.example.com/api/v1/repos/OWNER/REPO

curl -H "Authorization: token $TOKEN" \
  https://git.example.com/api/v1/repos/OWNER/REPO/archive/main.tar.gz \
  -o /tmp/repo.tar.gz
```

第二个请求返回 404 时，通常是分支不存在、仓库为空、owner/repo 写错或令牌无效。

### 日志提示“目标不存在”

服务无法下载仓库归档。常见原因：

- owner/repo 拼写错误。
- 分支不存在。
- 仓库为空。
- 子目录配置不影响归档下载，但配置错误会导致索引为空。

### 修改仓库后站点没变化

索引默认缓存 10 分钟。可以：

- 等待 `index.ttl_minutes`。
- 在管理后台点击“同步”。
- 如果希望写入后立即刷新，设置 `index.sync_on_write: true`。

### 仓库认证失败

令牌无效、过期或权限不足。重新生成具备目标仓库读写权限的令牌，在管理后台编辑来源并填入新令牌。

### 仓库服务不可用

服务端访问不到 Forgejo，或 TLS 证书有问题。先在服务器上执行：

```bash
curl -I https://git.example.com
```

## 编辑与保存

### 文档不存在

- 路径是否包含 `.md`。
- 文件是否真的在当前分支和 `root` 下。
- 匿名访问私有文档或草稿时会返回 404，这是预期行为。

### 内容已被他人修改

保存使用文件 SHA 做乐观锁。编辑期间文件被其他提交修改，需要刷新页面后重新编辑。

### 移动失败

- 目标路径是否已存在同名文件。
- 目标目录是否合法。
- 是否有写权限。
- 移动是“先创建再删除”，第二步失败时目标文件可能已经存在，需要手动检查仓库。

### 导入失败

- 只支持 `.md` 文件。
- 文件名不能包含 `/` 或 `\`。
- 当前目录下不能有同名文件。
- 需要 `editor` 或 `admin` 角色。

## 图片上传

### 图片存储未配置

`s3.endpoint` 或 `s3.bucket` 为空。补齐配置并重启。

### 不支持的文件类型

只支持 PNG、JPEG、GIF、WebP、SVG。检查：

- 扩展名是否在支持列表中。
- `s3.allow_types` 是否排除了该 MIME 类型。
- 文件内容是否与扩展名一致。把 `.png` 改成 `.jpg` 这类操作会被拒绝。

### 文件超过大小限制

调整 `s3.max_size_mb`，同时确认反向代理的 `client_max_body_size` 足够大。

### 上传成功但图片打不开

返回地址由 `s3.public_base` 拼接。检查：

- `public_base` 是否可从浏览器访问。
- 桶策略是否允许匿名读取。
- CDN 是否正确回源。
- 对象键前缀和实际写入位置是否一致。

## 构建与前端

### 构建时找不到前端产物

`go:embed all:dist` 需要 `internal/web/dist` 存在。先构建前端：

```bash
cd web
npm ci
npm run build
```

或直接使用 `bash ./build.sh`。

### 页面还是旧版本

前端已打进二进制，修改前端后必须重新构建并重启服务。浏览器缓存可以用强制刷新排除。

### Vite 开发页接口 404

确认后端运行在 `http://127.0.0.1:8080`，并且从 `web` 目录启动 `npm run dev`。代理配置在 `web/vite.config.ts`。

## 日志与健康检查

服务日志输出到标准输出。systemd 部署时：

```bash
journalctl -u gdocs -f
```

`log.debug: true` 会输出调试信息并启用 gin 的调试输出，`log.access: false` 可以关闭访问日志，业务日志仍会保留。

健康检查：

```bash
curl http://127.0.0.1:8080/healthz
```

`/healthz` 只检查进程和 PostgreSQL，不检查 Forgejo、OIDC 和 S3。后三者需要分别验证：

```bash
curl -I https://git.example.com
curl https://sso.example.com/.well-known/openid-configuration
curl -I https://s3.example.com
```
