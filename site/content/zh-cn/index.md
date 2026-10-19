---
title: mojang-yggdrasil-redirect
description: 为网络无法稳定访问 Mojang 的玩家与服务器提供的 Minecraft authlib 五项 Mojang 服务反向代理。
---

为网络无法稳定访问 Mojang 的玩家与服务器准备的反向代理，覆盖 Minecraft authlib 使用的五项 Mojang 服务。每项服务都通过 `yggdrasil.alisaqaq.moe` 的独立子域名提供，请求路径原样转发。

:::tip[提示]
公共实例已上线：`yggdrasil.alisaqaq.moe`，任何人都可以免费使用。只需添加下方的 JVM 参数即可，无需任何其他配置。想用自己的域名？请参阅[自行部署](#self-hosting)。
:::

## 快速开始 {#quick-start}

在 `-jar` 之前添加以下 JVM 参数（Minecraft 1.16+，客户端和服务端均适用）：

```
-Dminecraft.api.auth.host=https://auth.yggdrasil.alisaqaq.moe
-Dminecraft.api.account.host=https://account.yggdrasil.alisaqaq.moe
-Dminecraft.api.session.host=https://session.yggdrasil.alisaqaq.moe
-Dminecraft.api.services.host=https://services.yggdrasil.alisaqaq.moe
-Dminecraft.api.profiles.host=https://profiles.yggdrasil.alisaqaq.moe
```

单行版本：

```
-Dminecraft.api.auth.host=https://auth.yggdrasil.alisaqaq.moe -Dminecraft.api.account.host=https://account.yggdrasil.alisaqaq.moe -Dminecraft.api.session.host=https://session.yggdrasil.alisaqaq.moe -Dminecraft.api.services.host=https://services.yggdrasil.alisaqaq.moe -Dminecraft.api.profiles.host=https://profiles.yggdrasil.alisaqaq.moe
```

Velocity 代理只需要：

```
-Dmojang.sessionserver=https://session.yggdrasil.alisaqaq.moe/session/minecraft/hasJoined
```

## 端点列表 {#endpoints}

| 镜像地址 | 上游服务 | JVM 属性 | 说明 |
| --- | --- | --- | --- |
| `auth.yggdrasil.alisaqaq.moe` | `authserver.mojang.com` | `minecraft.api.auth.host` | 传统登录；MC 1.20.2 起不再使用 |
| `account.yggdrasil.alisaqaq.moe` | `api.mojang.com` | `minecraft.api.account.host` | 玩家信息；MC 1.20.3 起不再使用 |
| `session.yggdrasil.alisaqaq.moe` | `sessionserver.mojang.com` | `minecraft.api.session.host` | 在线模式进服验证 |
| `services.yggdrasil.alisaqaq.moe` | `api.minecraftservices.com` | `minecraft.api.services.host` | 聊天举报等；自 MC 1.16.4 起 |
| `profiles.yggdrasil.alisaqaq.moe` | `api.mojang.com` | `minecraft.api.profiles.host` | 玩家信息；自 authlib 7.0.61 起 |

## 工作原理 {#how-it-works}

主机名的第一个标签决定上游服务：对 `session.yggdrasil.alisaqaq.moe/session/minecraft/hasJoined` 的请求会被转发到 `sessionserver.mojang.com/session/minecraft/hasJoined`，路径与查询参数保持不变。未知端点返回 JSON 404；上游失败返回 JSON 502。

## 自行部署 {#self-hosting}

`yggdrasil.alisaqaq.moe` 只是其中一个部署实例——你完全可以在自己的域名上运行自己的实例。请将下文的 `yggdrasil.example.com` 替换为你的域名。

:::caution[重要]
请在出口 IP 未被 Mojang 屏蔽的主机上运行。Mojang API 部署在 Azure Front Door 之后，会拒绝来自部分云服务出口网段（例如 Cloudflare Workers）的请求——普通 VPS 或家庭宽带出口均可正常使用。部署到新主机前请先验证：

```sh
curl -s -o /dev/null -w '%{http_code}\n' https://sessionserver.mojang.com/session/minecraft/hasJoined   # 204 = OK
curl -s -o /dev/null -w '%{http_code}\n' https://api.minecraftservices.com/publickeys                    # 200 = OK
```
:::

从 GHCR 拉取已发布的镜像：

```sh
docker pull ghcr.io/alisaakiron/mojang-yggdrasil-redirect:latest
```

服务监听 `PORT`（默认 `8787`），并根据请求的 `Host` 选择上游，因此请将其部署在为 `*.yggdrasil.example.com` 终结 TLS 的反向代理之后。使用 Traefik 时，容器标签即可完成配置——Traefik 会终结 TLS、保留 Host 头并自动设置 `X-Forwarded-Proto`：

```yaml
# compose.yaml — assumes Traefik's Docker provider, a `websecure` entrypoint on :443,
# and that this container shares a Docker network with Traefik.
services:
  mojang-yggdrasil-redirect:
    image: ghcr.io/alisaakiron/mojang-yggdrasil-redirect:latest
    restart: unless-stopped
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.yggdrasil.rule=Host(`yggdrasil.example.com`) || Host(`*.yggdrasil.example.com`)"
      - "traefik.http.routers.yggdrasil.entrypoints=websecure"
      - "traefik.http.routers.yggdrasil.tls=true"
      - "traefik.http.services.yggdrasil.loadbalancer.server.port=8787"
```

TLS 证书必须覆盖二级通配符 `*.yggdrasil.example.com`（区域级的通配符证书 `*.example.com` 无法覆盖）——请通过 DNS-01 方式签发，例如添加 `traefik.http.routers.yggdrasil.tls.certresolver=<resolver>`。

想让实例只做纯代理？在容器环境中设置 `DISABLE_LANDING_PAGE=1`，基础域名将不再提供任何文档——`/` 会像未知端点一样返回 JSON 404。

想搭建私有实例，可设置 `PROXY_TOKEN=<secret>`（建议使用较长的 URL 安全字符串），并将其作为路径后缀追加到每个参数中——authlib 会在其后拼接请求路径：

```
-Dminecraft.api.session.host=https://session.yggdrasil.example.com/<secret>
```

代理会校验并移除第一个路径段；令牌错误或缺失时返回与未知端点相同的 JSON 404。`Authorization` 头原样透传，因此需要 Mojang Bearer 令牌的路由不受影响。配置令牌后文档站会自动禁用。令牌包含在请求 URL 中，请勿在反向代理中开启访问日志（参见[隐私](#privacy)）。

## 隐私 {#privacy}

转发前会移除可识别客户端的请求头（`CF-Connecting-IP`、`X-Forwarded-*` 等），因此 Mojang 只能看到本机的出口 IP。进程仅向 stdout 记录代理错误，不保留任何请求日志。注意：`hasJoined` 请求的 URL 中包含 `username` 与 `serverId` 查询参数——如果在意这一点，请避免在反向代理中开启详细的访问日志。

---

实现思路来自[这篇博客文章](https://blog.fallenbreath.me/zh-CN/2025/minecraft-service-proxy/)。
