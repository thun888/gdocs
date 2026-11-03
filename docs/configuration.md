# 配置参考

配置文件默认为仓库根目录的 `config.yaml`，可用 `-c` 指定其他路径：

```bash
gdocs -c /etc/gdocs/config.yaml
```

加载规则：

- 支持 `${ENV_NAME}` 环境变量展开，变量未设置会报错。
- 字段名严格校验，出现未知字段会拒绝启动。
- `config.yaml` 已被 `.gitignore` 忽略，真实凭据不要提交。
- 配置只在启动时读取，修改后需要重启服务。

## server

| 字段 | 默认值 | 说明 |
|---|---|---|
| `addr` | `:8080` | HTTP 监听地址 |
| `base_url` | 空 | 站点外部访问地址，如 `https://docs.example.com`。以 `https://` 开头时会话 Cookie 带 `Secure` |
| `session_key` | 空，必填 | 至少 32 字节，用于加密会话中的 OIDC 令牌和签名 Cookie |
| `public_read` | `false` | 全局默认匿名可见性。文档 front-matter 的 `public` 可覆盖 |
| `trusted_proxies` | 空 | 反向代理 IP 或 CIDR 列表，用于正确获取客户端 IP |

`session_key` 更换后，已有会话全部失效。

## database

| 字段 | 默认值 | 说明 |
|---|---|---|
| `dsn` | 空，必填 | PostgreSQL 连接串 |
| `max_open` | `20` | 最大连接数 |
| `max_idle` | `5` | 空闲连接数 |
| `auto_migrate` | `false` | 启动时自动执行迁移 |

生产环境建议保持 `auto_migrate: false`，升级时手动执行：

```bash
gdocs -c /etc/gdocs/config.yaml -migrate
```

## oauth

| 字段 | 默认值 | 说明 |
|---|---|---|
| `issuer` | 空，必填 | OIDC issuer，必须与发现文档中的 `issuer` 完全一致 |
| `client_id` | 空，必填 | OIDC 客户端 ID |
| `client_secret` | 空，必填 | OIDC 客户端密钥 |
| `redirect_url` | 空，必填 | 必须是绝对地址，指向 `/api/v1/auth/callback` |
| `scopes` | `openid profile email groups` | 请求的 scope |
| `groups_claim` | `groups` | 预留字段，当前版本固定读取 `groups` 声明 |
| `bootstrap_admin` | 空 | 管理员豁免名单，按 OIDC `sub` 精确匹配 |

回调地址示例：

```yaml
oauth:
  redirect_url: "https://docs.example.com/api/v1/auth/callback"
```

该地址必须与 OIDC 后台登记的回调完全一致。要使用用户组权限，OIDC 应用必须被允许返回 `groups` scope，否则所有用户都会是 `viewer`。

`bootstrap_admin` 里的账号不受 `access.allowed_groups` 限制，并且角色固定为 `admin`。

## access

| 字段 | 默认值 | 说明 |
|---|---|---|
| `allowed_groups` | 空 | 允许登录的用户组。空表示不限制 |
| `deny_message` | 空 | 用户组不匹配时返回的提示 |

匹配规则：

- 与 `groups` 声明中的 code 精确匹配。
- 以 `*` 结尾时按前缀匹配，例如 `unit:学生/*`。
- 命中任意一条即可登录。

示例：

```yaml
access:
  allowed_groups:
    - "user_type:学生"
    - "unit:学生/计算机学院*"
  deny_message: "当前账号不在 gdocs 的访问范围内"
```

`allowed_groups` 只限制登录，不影响 `public_read` 下的匿名阅读。

## git

| 字段 | 默认值 | 说明 |
|---|---|---|
| `bot_name` | 空 | 没有登录用户时的提交者名称 |
| `bot_email` | 空，必填 | 用户邮箱缺失时的兜底提交邮箱 |

正常编辑时，提交作者和提交者都使用当前登录用户的显示名称与邮箱；邮箱为空时回落到 `bot_email`。

## index

| 字段 | 默认值 | 说明 |
|---|---|---|
| `ttl_minutes` | `10` | 目录索引自动刷新间隔，单位分钟 |
| `sync_on_write` | `false` | 站点写入后是否立即重建索引 |
| `max_archive_mb` | `50` | 单次下载仓库归档的大小上限，单位 MB |

索引通过下载仓库归档并解析 Markdown 的 front-matter 建立。仓库很大时适当调高 `max_archive_mb`。

`sync_on_write: true` 能让编辑后立刻看到目录变化，但每次保存都会多一次归档下载。

## s3

| 字段 | 默认值 | 说明 |
|---|---|---|
| `endpoint` | 空 | S3 兼容服务地址，`endpoint` 或 `bucket` 为空时禁用上传 |
| `region` | 空 | 区域，如 `us-east-1` |
| `bucket` | 空 | 桶名 |
| `access_key` | 空 | 访问密钥 |
| `secret_key` | 空 | 私钥 |
| `path_prefix` | 空 | 对象键前缀，如 `docs/` |
| `public_base` | 空 | 图片公开访问前缀，必须能被浏览器直接访问 |
| `max_size_mb` | `10` | 单张图片大小上限 |
| `allow_types` | 空 | 允许的 MIME 类型，空表示不额外限制 |

支持的扩展名固定为 `.png`、`.jpg`、`.jpeg`、`.gif`、`.webp`、`.svg`。上传时会校验文件内容与扩展名是否一致。

对象键格式：

```text
{path_prefix}/{YYYY}/{MM}/{内容哈希前16位}{扩展名}
```

返回给编辑器的地址为 `{public_base}/{对象键}`。`public_base` 为空时返回站内相对路径，通常无法直接访问，因此生产环境应配置公开读的桶或 CDN 地址。

SVG 以附件方式写入，避免直接在浏览器中执行脚本。

## sources

`sources` 是仓库来源列表，只在启动时用于初始化数据库。已存在的 slug 不会被配置覆盖；新增、修改、停用和删除来源应在[管理后台](administration.md)操作。

| 字段 | 默认值 | 说明 |
|---|---|---|
| `slug` | 空，必填 | 来源标识，只允许小写字母、数字和连字符 |
| `name` | 空，必填 | 展示名称 |
| `base_url` | 空，必填 | Forgejo 地址，如 `https://git.example.com` |
| `token` | 空，必填 | 仓库访问令牌，需具备目标仓库读写权限 |
| `owner` | 空，必填 | 仓库所有者 |
| `repo` | 空，必填 | 仓库名 |
| `branch` | `main` | 分支名 |
| `root` | 空 | 仓库内的子目录，空表示仓库根目录 |
| `sort` | `0` | 来源排序，数值小的在前 |

示例：

```yaml
sources:
  - slug: "handbook"
    name: "员工手册"
    base_url: "https://git.example.com"
    token: "${FORGEJO_TOKEN_HANDBOOK}"
    owner: "docs"
    repo: "handbook"
    branch: "main"
    root: "content"
    sort: 10
```

注意：

- `token` 会使用 `session_key` 加密后写入数据库。
- 仓库必须至少有一个提交，否则无法下载归档。
- 从管理后台删除由配置初始化的来源后，重启服务会再次按配置创建。需要永久删除时，同时从配置中移除。
- 同一仓库可以用不同 `slug` 和 `root` 暴露成多个来源。

## 最小示例

```yaml
server:
  addr: ":8080"
  base_url: "http://127.0.0.1:8080"
  session_key: "${GDOCS_SESSION_KEY}"
  public_read: true

database:
  dsn: "postgres://gdocs:gdocs@127.0.0.1:5432/gdocs?sslmode=disable"

oauth:
  issuer: "https://sso.example.com"
  client_id: "gdocs"
  client_secret: "${OAUTH_CLIENT_SECRET}"
  redirect_url: "http://127.0.0.1:8080/api/v1/auth/callback"
  scopes: ["openid", "profile", "email", "groups"]
  bootstrap_admin: ["20230001"]

git:
  bot_name: "gdocs"
  bot_email: "gdocs@example.com"

sources:
  - slug: "handbook"
    name: "员工手册"
    base_url: "https://git.example.com"
    token: "${FORGEJO_TOKEN_HANDBOOK}"
    owner: "docs"
    repo: "handbook"
```

更多字段的占位示例见仓库根目录的 `config.example.yaml`。
