# 构建与部署

## 构建

推荐使用仓库自带的脚本：

```bash
bash ./build.sh
```

脚本会：

1. `web/node_modules` 不存在时执行 `npm ci`。
2. 在 `web` 下执行 `npm run build`，产物写入 `internal/web/dist`。
3. 编译 Go 程序到 `bin/gdocs`。

交叉编译：

```bash
bash ./build.sh -t linux/amd64
bash ./build.sh -t linux/arm64
bash ./build.sh -t windows/amd64
```

Windows 目标会生成 `bin/gdocs.exe`。

前端产物和数据库迁移都通过 `go:embed` 打进二进制，运行时只需要：

```text
gdocs
config.yaml
```

## 运行

首次部署先执行迁移：

```bash
./gdocs -c /etc/gdocs/config.yaml -migrate
```

然后启动：

```bash
./gdocs -c /etc/gdocs/config.yaml
```

默认监听 `:8080`。生产环境建议设置：

```bash
export GIN_MODE=release
```

减少调试路由日志。

## systemd

```ini
[Unit]
Description=gdocs
After=network.target postgresql.service

[Service]
Type=simple
User=gdocs
Group=gdocs
WorkingDirectory=/opt/gdocs
ExecStart=/opt/gdocs/gdocs -c /etc/gdocs/config.yaml
Environment=GIN_MODE=release
Restart=on-failure
RestartSec=3

[Install]
WantedBy=multi-user.target
```

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now gdocs
sudo systemctl status gdocs
```

配置文件建议限制权限：

```bash
sudo chown gdocs:gdocs /etc/gdocs/config.yaml
sudo chmod 600 /etc/gdocs/config.yaml
```

## Nginx 反向代理

```nginx
server {
    listen 443 ssl http2;
    server_name docs.example.com;

    ssl_certificate     /etc/ssl/docs.example.com.pem;
    ssl_certificate_key /etc/ssl/docs.example.com.key;

    client_max_body_size 20m;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

对应配置：

```yaml
server:
  base_url: "https://docs.example.com"
  trusted_proxies: ["127.0.0.1"]
```

OIDC 回调：

```yaml
oauth:
  redirect_url: "https://docs.example.com/api/v1/auth/callback"
```

要点：

- `base_url` 使用 `https://` 时，会话 Cookie 才会带 `Secure`。
- 代理不在本机时，`trusted_proxies` 填写实际代理 IP 或网段。
- `client_max_body_size` 要大于图片大小上限，给表单开销留余量。
- 健康检查地址为 `/healthz`。

### tinyauth 前置认证

用 tinyauth 接管登录时，让它把身份头写进转发给 gdocs 的请求：

```nginx
location / {
    auth_request /tinyauth;

    auth_request_set $remote_user   $upstream_http_remote_user;
    auth_request_set $remote_sub    $upstream_http_remote_sub;
    auth_request_set $remote_name   $upstream_http_remote_name;
    auth_request_set $remote_email  $upstream_http_remote_email;
    auth_request_set $remote_groups $upstream_http_remote_groups;

    proxy_set_header Remote-User   $remote_user;
    proxy_set_header Remote-Sub    $remote_sub;
    proxy_set_header Remote-Name   $remote_name;
    proxy_set_header Remote-Email  $remote_email;
    proxy_set_header Remote-Groups $remote_groups;

    proxy_pass http://127.0.0.1:8080;
    proxy_set_header Host $host;
    proxy_set_header X-Real-IP $remote_addr;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
    proxy_set_header X-Forwarded-Proto $scheme;
}

location = /tinyauth {
    internal;
    proxy_pass http://tinyauth:3000/api/auth/nginx;
    proxy_pass_request_body off;
    proxy_set_header Content-Length "";
    proxy_set_header X-Original-URL $scheme://$http_host$request_uri;
    proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
}

error_page 401 = @tinyauth_login;
location @tinyauth_login {
    return 302 https://auth.example.com/login?redirect_uri=$scheme://$http_host$request_uri;
}
```

Traefik 在 forwardauth 的 `authResponseHeaders` 里列出 `remote-user,remote-sub,remote-name,remote-email,remote-groups`；Caddy 使用 `copy_headers remote-user remote-sub remote-name remote-email remote-groups`。

对应配置：

```yaml
server:
  trusted_proxies: ["127.0.0.1"]

proxy_auth:
  enabled: true
  login_url: "https://auth.example.com/login?redirect_uri={redirect}"
  logout_url: "https://auth.example.com/logout"
```

要点：

- 身份头只在请求来自 `trusted_proxies` 时生效。gdocs 不能暴露在绕过代理的地址上，否则可以直接伪造 `Remote-User`。
- 上面的 `proxy_set_header` 会覆盖客户端自带的同名头。使用别的代理时，确认它同样会覆盖，而不是透传。
- 要拿到用户组，tinyauth 需要从 OIDC 或 LDAP 取到 `groups` 并转发 `Remote-Groups`；没有这一项时按库里已有角色处理。
- 在 tinyauth 上切换账号后，gdocs 最多 `cache_seconds` 秒后跟着切换。

## 对象存储

图片由服务端直接写入 S3，浏览器通过 `s3.public_base` 读取：

```yaml
s3:
  endpoint: "https://s3.example.com"
  bucket: "gdocs"
  path_prefix: "docs/"
  public_base: "https://cdn.example.com/gdocs"
```

需要保证：

- 桶对服务端账号可写。
- `public_base` 指向的对象可被匿名读取。
- 使用 MinIO 时，桶策略允许公开读取，或通过 CDN 回源。
- SVG 以附件方式上传，避免脚本执行。

没有图片需求时可以把 `endpoint` 和 `bucket` 留空，上传接口会返回“图片存储未配置”，其他功能不受影响。

## 数据库

`docker-compose.yml` 只用于本地运行 PostgreSQL：

```bash
docker compose up -d postgres
```

生产环境建议使用独立 PostgreSQL 16，并配置备份。

迁移策略：

- 常规升级：停止服务 → 备份数据库 → 执行 `-migrate` → 替换二进制 → 启动。
- `-migrate` 只执行迁移后退出，不启动服务。
- `database.auto_migrate: true` 会在启动时先迁移再继续启动，适合测试环境；生产环境建议显式控制迁移时机。

## 升级

```bash
# 1. 备份数据库和配置
pg_dump ... > backup.sql
sudo cp /etc/gdocs/config.yaml /etc/gdocs/config.yaml.bak

# 2. 替换二进制
sudo install -m 0755 gdocs /opt/gdocs/gdocs

# 3. 执行迁移
sudo -u gdocs /opt/gdocs/gdocs -c /etc/gdocs/config.yaml -migrate

# 4. 重启
sudo systemctl restart gdocs

# 5. 检查
curl https://docs.example.com/healthz
```

`server.session_key` 保持不变，否则所有用户需要重新登录。OAuth、S3、Git 凭据也不要随升级丢失。

## 备份

需要备份的内容：

| 内容 | 位置 |
|---|---|
| 用户、组、来源、索引、设置 | PostgreSQL |
| 文档正文和历史 | Forgejo 仓库 |
| 图片 | S3 桶 |
| 运行配置 | `config.yaml` |

正文只存在 Git 中，数据库丢失不会丢文档，但会丢失用户、组、来源和排序索引。恢复后重新同步来源即可重建索引。

## 上线检查

- [ ] `config.yaml` 权限为 600，未提交到仓库。
- [ ] `server.base_url` 和 `oauth.redirect_url` 使用正式 HTTPS 域名。
- [ ] OIDC 后台已登记回调地址，并允许返回 `groups` scope。
- [ ] `session_key` 为随机生成的 32 字节以上字符串。
- [ ] `access.allowed_groups` 按需要限制登录范围。
- [ ] `bootstrap_admin` 至少包含一个可用管理员。
- [ ] 仓库令牌只授权目标仓库，且具备读写权限。
- [ ] 仓库目标分支至少有一个提交。
- [ ] S3 桶可写，`public_base` 可匿名读取。
- [ ] PostgreSQL、Git、S3 均有备份策略。
