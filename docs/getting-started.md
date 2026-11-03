# 快速开始

## 准备环境

| 依赖 | 用途 |
|---|---|
| Go 1.27 或更高 | 编译后端 |
| Node.js 20 或更高、npm | 编译前端 |
| PostgreSQL 16 | 用户、会话、来源、文档索引 |
| Forgejo 或 Gitea | 存放 Markdown 正文 |
| OIDC 服务 | 用户登录 |
| S3 兼容存储 | 图片上传，可选 |

仓库自带 `docker-compose.yml`，只包含 PostgreSQL，便于本地启动：

```bash
docker compose up -d postgres
```

已有 PostgreSQL 时跳过这一步，创建数据库和账号即可。

## 1. 准备配置

```bash
cp config.example.yaml config.yaml
```

至少填写以下内容：

- `server.session_key`：至少 32 字节的随机值。可以用 `openssl rand -base64 32` 生成。
- `database.dsn`：PostgreSQL 连接串。
- `oauth`：issuer、client_id、client_secret、redirect_url。
- `sources`：至少一个仓库来源。

完整字段见[配置参考](configuration.md)。

`config.yaml` 里包含真实凭据，不要提交。示例配置使用 `${ENV_NAME}` 占位符，启动时会展开；环境变量缺失会直接报错。

## 2. 执行数据库迁移

```bash
go run ./cmd/gdocs -migrate
```

迁移文件已嵌入二进制，部署时不需要额外复制 `migrations` 目录。

也可以设置 `database.auto_migrate: true`，让服务每次启动时自动迁移。生产环境更建议手动执行 `-migrate`。

## 3. 启动服务

开发时直接运行：

```bash
go run ./cmd/gdocs -c config.yaml
```

或者先构建再运行：

```bash
bash ./build.sh
./bin/gdocs -c config.yaml
```

Windows 下构建产物是 `bin/gdocs.exe`：

```powershell
go build -o bin/gdocs.exe ./cmd/gdocs
.\bin\gdocs.exe -c config.yaml
```

默认监听 `:8080`，打开 <http://127.0.0.1:8080>。

检查健康状态：

```bash
curl http://127.0.0.1:8080/healthz
```

正常返回：

```json
{"db":"ok","status":"ok"}
```

## 4. 登录并设置第一个管理员

点击页面右上角“登录账户”，会跳转到 OIDC 服务。

新用户默认角色是 `viewer`，只能浏览。要获得第一个管理员，把登录用户的 `sub`（通常是学号或工号）填进配置：

```yaml
oauth:
  bootstrap_admin: ["20230001"]
```

重启服务后重新登录，该账号角色会变为 `admin`。之后可以在[管理后台](administration.md)给用户组分配 `editor` 或 `admin`。

## 5. 放入第一篇文档

在 Forgejo 仓库的目标分支推送一个 Markdown 文件，例如 `README.md`：

```markdown
---
title: 首页
---

# 欢迎

这是第一篇文档。
```

服务启动时会预热索引，之后每次访问目录树也会按 `index.ttl_minutes` 自动刷新。也可以在管理后台点击“同步”立即重建索引。

注意：仓库必须至少有一个提交。空仓库没有可下载的归档，索引会提示“目标不存在”。

## 前端开发

后端和前端可以分开运行：

```bash
# 终端 1
go run ./cmd/gdocs -c config.yaml

# 终端 2
cd web
npm run dev
```

Vite 开发服务器默认在 <http://127.0.0.1:5173>，并把 `/api` 代理到 `http://127.0.0.1:8080`。

## 下一步

- 了解文档目录、front-matter 和排序：[文档仓库规范](content.md)
- 了解编辑、导入和历史：[站点使用](user-guide.md)
- 部署到服务器：[构建与部署](deployment.md)
