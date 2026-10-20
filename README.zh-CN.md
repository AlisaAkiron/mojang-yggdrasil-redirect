# mojang-yggdrasil-redirect

[English](README.md) | 简体中文

一个 Go 反向代理，覆盖 Minecraft authlib 使用的五项 Mojang 服务，供网络无法稳定访问
Mojang 的玩家与服务器使用。实现思路来自[这篇博客文章](https://blog.fallenbreath.me/zh-CN/2025/minecraft-service-proxy/)。

每项服务都通过独立的子域名提供：主机名的第一个标签决定上游服务，请求路径原样转发。

> [!TIP]
> **公共实例已上线：[`yggdrasil.alisaqaq.moe`](https://yggdrasil.alisaqaq.moe/)**——部署在我自己的服务器上，
> 任何人都可以免费使用。访问该网站即可查看端点列表和可直接复制的配置说明（JVM 参数、authlib-injector、
> Prism Launcher）。想自己运行？请参阅[自行部署](#自行部署-docker)，使用你自己的域名部署。

## 开发

需要 Go 1.27+（`nix develop` / direnv 会自动提供）。

```sh
go generate ./site  # Markdown 文档 -> site/dist（嵌入到 api 二进制中）
go run ./cmd/api    # http://localhost:8787
go test ./...       # 单元测试
go build ./cmd/api  # 单个静态二进制文件，已包含文档
```

`HOST` 和 `PORT` 用于设置监听地址（默认 `0.0.0.0:8787`）。

文档是 `site/content/` 下的 Markdown 文件，开头为包含 `title`/`description` 的 frontmatter。
除 GitHub 风格的表格外，生成器还支持显式标题锚点（`## 自行部署 {#self-hosting}`，让所有语言共用
相同的 `#id`）以及提示块：

```md
:::tip[可选标题]
Markdown 内容
:::
```

编辑后请重新运行 `go generate ./site`，然后重启 api。`site/dist` 在编译时嵌入，因此未先生成就构建的
二进制不会提供任何文档。公共域名直接写在文档内容中，运行时服务端会将其替换为当前访问的域名，
因此自行部署的实例也能显示正确的地址。添加新语言 = 新增 `site/content/<dir>/index.md`，并在
`site/config.json` 的 `locales` 中添加一项；各语言中同名的页面会自动互相链接。

通过 Host 头在本地测试子域名：

```sh
curl -s http://localhost:8787/session/minecraft/hasJoined -H 'Host: session.yggdrasil.localhost'   # 204
```

## 自行部署 (Docker)

`yggdrasil.alisaqaq.moe` 只是我的部署实例——你完全可以在自己的域名上运行自己的实例。请将下文的
`yggdrasil.example.com` 替换为你的域名。

> [!IMPORTANT]
> **请在出口 IP 未被 Mojang 屏蔽的主机上运行。** Mojang API 部署在 Azure Front Door 之后，
> 对来自 Cloudflare Workers 出口 IP 的请求会返回 `403 The request is blocked`——这正是本项目是
> 一个需要自行部署（VPS / 家用服务器）的普通服务、**而不是** Cloudflare Worker 的原因。普通 VPS
> 或家庭宽带出口均可正常使用。部署到新主机前请先验证：
> ```sh
> curl -s -o /dev/null -w '%{http_code}\n' https://sessionserver.mojang.com/session/minecraft/hasJoined   # 204 = OK
> curl -s -o /dev/null -w '%{http_code}\n' https://api.minecraftservices.com/publickeys                    # 200 = OK
> ```

服务监听 `PORT`（默认 `8787`），并根据请求的 `Host` 选择上游，因此请将其部署在为
`*.<你的域名>` 终结 TLS 的反向代理之后。

从 GHCR 拉取已发布的镜像：

```sh
docker pull ghcr.io/alisaakiron/mojang-yggdrasil-redirect:latest
```

使用容器标签将其部署在 Traefik 之后——Traefik 会终结 TLS、保留 Host 头并自动设置
`X-Forwarded-Proto`：

```yaml
# compose.yaml — assumes Traefik's Docker provider, a `websecure` entrypoint on :443,
# and that this container shares a Docker network with Traefik.
services:
  mojang-yggdrasil-redirect:
    image: ghcr.io/alisaakiron/mojang-yggdrasil-redirect:latest
    restart: unless-stopped
    environment:
      - "ENABLE_MODULE=yggdrasil,authlib-injector,prism-meta"
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.yggdrasil.rule=Host(`yggdrasil.example.com`) || Host(`*.yggdrasil.example.com`)"
      - "traefik.http.routers.yggdrasil.entrypoints=websecure"
      - "traefik.http.routers.yggdrasil.tls=true"
      - "traefik.http.services.yggdrasil.loadbalancer.server.port=8787"
```

TLS 证书必须覆盖二级通配符 `*.yggdrasil.example.com`（区域级的通配符证书 `*.example.com`
无法覆盖）——请通过 DNS-01 方式签发，例如添加
`traefik.http.routers.yggdrasil.tls.certresolver=<resolver>`。

每项功能都是一个模块，**所有模块默认关闭**——在 `ENABLE_MODULE` 中以逗号分隔列出需要启用的模块：

| 模块 | 子域名 |
| --- | --- |
| `yggdrasil` | `auth`、`account`、`session`、`services`、`profiles` |
| `authlib-injector` | `authlib-injector` |
| `prism-meta` | `prism-meta` |

未启用模块的子域名返回 JSON 404；未启用任何模块或模块名写错时，服务会在启动时报错退出。

想让实例只做纯代理？在容器环境中设置 `DISABLE_LANDING_PAGE=1`，基础域名将不再提供任何文档——
`/` 会像未知端点一样返回 JSON 404。

想搭建私有实例，可设置 `PROXY_TOKEN=<secret>`（请选择较长的 URL 安全字符串），并将其作为路径
后缀加入每个参数——authlib 会在其后拼接请求路径：

```
-Dminecraft.api.auth.host=https://auth.yggdrasil.example.com/<secret>
-Dminecraft.api.account.host=https://account.yggdrasil.example.com/<secret>
-Dminecraft.api.session.host=https://session.yggdrasil.example.com/<secret>
-Dminecraft.api.services.host=https://services.yggdrasil.example.com/<secret>
-Dminecraft.api.profiles.host=https://profiles.yggdrasil.example.com/<secret>
```

代理会校验第一个路径段（使用防时序攻击的比较）、将其移除，再转发剩余部分；令牌错误或缺失时返回
与未知端点相同的 JSON 404。`Authorization` 头原样透传，因此需要 Mojang Bearer 令牌的路由
（玩家证书、聊天举报）不受影响。配置令牌后落地页会自动禁用。由于令牌包含在请求 URL 中，请关闭
反向代理的访问日志（参见[隐私](#隐私)）。

## 隐私

转发前会移除可识别客户端的请求头（`CF-Connecting-IP`、`X-Forwarded-*` 等），因此 Mojang
只能看到本机的出口 IP。进程仅向 stdout 记录代理错误（一行 JSON 格式的 `proxy_error`），不保留
任何请求日志。注意：`hasJoined` 请求的 URL 中包含 `username`/`serverId` 查询参数——如果在意
这一点，请避免在反向代理中开启详细的访问日志。
