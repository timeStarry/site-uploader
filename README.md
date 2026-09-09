# site-uploader

极简静态 HTML 上传与浏览服务。上传需要 Bearer 令牌；首页、索引和上传的页面允许直接读取。

首次配置可用 `GET /api/auth/check` 搭配 Bearer 令牌做无副作用的凭据检查；有效令牌返回 HTTP 204，缺失或错误令牌返回 401。

## 部署拓扑

应用部署在本机 Docker，只发布 `100.99.0.5:18080`（Tailnet 地址）。Caddy 部署在 `timestarry@100.99.0.6`，通过 Tailnet 反代到本机。域名入口为 `https://site.tsio.top`。

`18080` 大于 10000，部署前已确认空闲。Compose 不向其他网卡发布应用端口。域名入口开放给哪些客户端，由 Caddy 入口和网络访问策略决定；Tailnet 限制的是这里的上游连接。

## 本机运行

已有 `.env` 含上传令牌，请保留。新环境复制 `.env.example` 并设置随机 `UPLOAD_TOKEN`；不要把真实令牌写入代码或文档。

```sh
mkdir -p data
chmod 600 .env
docker compose config --quiet
docker compose up -d --build
docker compose ps
docker compose logs --tail=30
curl --noproxy '*' -f http://100.99.0.5:18080/
```

持久化目录为 `./data`，容器内为 `/data`。默认以 `1000:1000` 运行，与本机数据目录属主一致；其他环境可在 `.env` 设置 `APP_UID` / `APP_GID`。`BIND_IP` 默认为 `100.99.0.5`，`PORT` 默认为 `18080`。容器配置了 `unless-stopped` 重启策略。

Dockerfile 使用本机已缓存的 DaoCloud Go 1.26 / Debian bookworm 镜像，以避开当前 Docker Hub 连接超时。

## 远端 Caddy 与 DNS

站点配置见 `deploy/site.tsio.top.Caddyfile`。远端 Caddy 从 `/etc/caddy/Caddyfile` 启动；仅修改管理 API 不会在服务重启后保留。

2026-09-09 已准备并验证远端目录：

```text
/home/timestarry/caddy_config/site-uploader-20260909T084039Z
```

目录内包含原始 Caddyfile、运行配置备份、候选配置、校验信息和安装脚本。候选配置保留所有现有运行路由，并把此前只存在于运行配置中的 `thmk.tsio.top` 路由保存到文件。

安装需要远端 sudo 交互认证：

```sh
ssh -t timestarry@100.99.0.6 'sudo bash /home/timestarry/caddy_config/site-uploader-20260909T084039Z/install-caddy.sh /home/timestarry/caddy_config/site-uploader-20260909T084039Z'
```

脚本检查准备后配置有无变化，验证候选配置，备份并安装文件，再 reload；reload 失败时恢复文件和原运行配置。若脚本报告配置发生变化，需要重新生成候选文件，不要跳过检查。

权威 DNS 当前对 `site.tsio.top` 返回 NXDOMAIN。为使公网域名入口到达该 Caddy，可添加 `A site → 39.102.215.160`（DNS only，与 `duallane.tsio.top` 使用同一入口）。DNS 生效且 Caddy 配置安装后，由 Caddy 自动签发 HTTPS 证书。不要将公网 DNS 上游设置成本机应用地址。

```sh
curl -I https://site.tsio.top/
curl -f https://site.tsio.top/api/sites
```

## 上传

从 `.env` 读取 `UPLOAD_TOKEN` 并作为 Bearer 令牌调用接口：

```sh
curl -X POST https://site.tsio.top/api/sites \
  -H 'Authorization: Bearer YOUR_TOKEN' \
  -F 'title=我的页面' -F 'summary=页面摘要' -F 'file=@index.html'
```

DNS 和 Caddy 完成前，可在 Tailnet 内用 `http://100.99.0.5:18080` 调用相同接口。接口返回的页面 URL 仍使用配置的域名。

## 已验证状态（2026-09-09）

- Go 编译检查通过；项目当前无自动测试文件。
- Docker 构建启动通过，绑定 `100.99.0.5:18080`，重启计数为 0。
- 本机首页、未授权上传 401、授权上传、页面读取和容器重启后的持久化均通过；测试上传已清理。
- Caddy 服务器通过 Tailnet 读取应用成功，候选配置通过 `caddy validate`。
- 远端 Caddy 配置已安装并加载 `site.tsio.top`；DNS A 记录已生效，Caddy 已签发 Let's Encrypt 证书。
- 若某些公网网络仍出现 SSL 重置或 502，优先检查该网络到阿里云公网入口的链路；Caddy 本机和 Tailnet 上游验证正常。

## Codex skill

`skills/site-uploader` 是配套 Codex skill。它首次使用时检查 `${CODEX_HOME:-$HOME/.codex}/secrets.env`，通过 `/api/auth/check` 验证令牌，再上传用户指定的 HTML；令牌不会写入仓库或输出。
