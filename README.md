# site-uploader

一个可独立部署的静态 HTML 托管服务。首页居中显示可配置标题、站点域名和页面数量；页面列表与搜索需要 Bearer 令牌。

## 配置与运行

复制 `.env.example` 为 `.env` 并设置随机 `UPLOAD_TOKEN`。通过 `DOMAIN`、`PORT`、`HOST_PORT`、`BIND_IP`、`DATA_DIR` 和 `SITE_TITLE` 配置运行环境；不要提交 `.env` 或运行数据。

```sh
cp .env.example .env
chmod 600 .env
docker compose config --quiet
docker compose up -d --build
docker compose ps
```

Compose 默认仅绑定回环地址，端口仅为示例值；生产部署应由部署环境选择可用地址、端口和反向代理。应用本体不依赖 Docker、反向代理、特定网络、特定用户目录或特定云服务，任何反向代理只需转发到配置的应用地址。

数据保存在 `DATA_DIR`，上传内容和索引不会自动迁移或删除。容器可通过 `APP_UID` / `APP_GID` 匹配宿主机目录属主。

## API

- `GET /api/auth/check`：Bearer 令牌检查；有效返回 `204`，无效返回 `401`。
- `GET /api/sites?page=1&page_size=20`：鉴权分页列表，最多每页 100 项。返回 `{items,page,page_size,total,pages}`。
- `GET /api/sites/search?q=...&page=1&page_size=20`：鉴权模糊搜索标题、摘要和 ID，使用简单本地分词，返回同样的分页结构。
- `POST /api/sites`：鉴权 multipart 上传。必填 `title`、`file`；可选 `summary`、`access_password`。访问密码必须是四位 ASCII 数字，省略即不设密码。单文件上限为 64 MiB，超限返回 413，不会静默截断。
- `PUT /api/sites/{id}`：鉴权替换已有页面文件；`file` 必填，其他字段可选，用于修复或更新页面并保留原 ID。
- `GET /s/{id}`：读取页面；受保护页面显示密码表单，正确提交后设置 HttpOnly Cookie。

上传示例：

```sh
curl -fsS -X POST "$BASE_URL/api/sites" \
  -H "Authorization: Bearer $UPLOAD_TOKEN" \
  -F 'title=我的页面' -F 'summary=页面摘要' -F 'file=@index.html'
```

## Codex skill

`skills/site-uploader` 提供 Agent 使用契约：首次使用检查 `${CODEX_HOME:-$HOME/.codex}/secrets.env`，再调用 `/api/auth/check` 验证令牌；列表和搜索必须携带令牌并遵守分页契约。默认不设置访问密码，只有用户明确提出时才传 `access_password`。

## 开发检查

```sh
gofmt -w main.go
go test ./...
go vet ./...
```

CI 会运行格式、测试、vet、构建和 Shell 语法检查。
