---
title: mojang-yggdrasil-redirect
description: Minecraft authlib 所需 Mojang 服务的镜像，以及使用 BMCLAPI 下载的 Prism Launcher 元数据——为网络无法稳定访问 Mojang 的玩家与服务器准备。
---

为网络无法稳定访问 Mojang 的玩家与服务器准备的镜像：Minecraft authlib 使用的五项 Mojang 服务、面向旧版本的 [authlib-injector](https://github.com/yushijinhun/authlib-injector) 端点，以及从 BMCLAPI 下载的 Prism Launcher 元数据。每项功能都通过 `yggdrasil.alisaqaq.moe` 的独立子域名提供。

:::tip[提示]
公共实例已上线：`yggdrasil.alisaqaq.moe`，任何人都可以免费使用，无需任何配置。请从下方选择对应的指南。想用自己的域名？请参阅[自行部署](#self-hosting)。
:::

## 使用指南 {#guides}

| 指南 | 适用场景 | 子域名 |
| --- | --- | --- |
| [Mojang 服务](/zh-cn/yggdrasil/) | Minecraft 1.16+ 客户端与服务端、Velocity | `auth`、`account`、`session`、`services`、`profiles` |
| [authlib-injector](/zh-cn/authlib-injector/) | Minecraft 1.7.10 – 1.15（例如 GT New Horizons） | `authlib-injector` |
| [Prism Launcher](/zh-cn/prism-launcher/) | 在 Prism 中加速下载游戏、依赖库和模组加载器 | `prism-meta` |

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
    environment:
      - "ENABLE_MODULE=yggdrasil,authlib-injector,prism-meta"
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.yggdrasil.rule=Host(`yggdrasil.example.com`) || Host(`*.yggdrasil.example.com`)"
      - "traefik.http.routers.yggdrasil.entrypoints=websecure"
      - "traefik.http.routers.yggdrasil.tls=true"
      - "traefik.http.services.yggdrasil.loadbalancer.server.port=8787"
```

TLS 证书必须覆盖二级通配符 `*.yggdrasil.example.com`（区域级的通配符证书 `*.example.com` 无法覆盖）——请通过 DNS-01 方式签发，例如添加 `traefik.http.routers.yggdrasil.tls.certresolver=<resolver>`。

每项功能都是一个模块，**所有模块默认关闭**——在 `ENABLE_MODULE` 中以逗号分隔列出需要启用的模块：

| 模块 | 子域名 |
| --- | --- |
| [`yggdrasil`](/zh-cn/yggdrasil/) | `auth`、`account`、`session`、`services`、`profiles` |
| [`authlib-injector`](/zh-cn/authlib-injector/) | `authlib-injector` |
| [`prism-meta`](/zh-cn/prism-launcher/) | `prism-meta` |

未启用模块的子域名返回 JSON 404；未启用任何模块或模块名写错时，服务会在启动时报错退出。

想让实例只做纯代理？在容器环境中设置 `DISABLE_LANDING_PAGE=1`，基础域名将不再提供任何文档——`/` 会像未知端点一样返回 JSON 404。

想搭建私有实例，可设置 `PROXY_TOKEN=<secret>`（建议使用较长的 URL 安全字符串）。此后每个请求都必须以令牌作为第一个路径段——各指南中都说明了令牌的位置。代理会校验并移除该路径段；令牌错误或缺失时返回与未知端点相同的 JSON 404。配置令牌后文档站会自动禁用。令牌包含在请求 URL 中，请勿在反向代理中开启访问日志（参见[隐私](#privacy)）。

## 隐私 {#privacy}

转发前会移除可识别客户端的请求头（`CF-Connecting-IP`、`X-Forwarded-*` 等），因此 Mojang 只能看到本机的出口 IP。进程仅向 stdout 记录代理错误，不保留任何请求日志。注意：`hasJoined` 请求的 URL 中包含 `username` 与 `serverId` 查询参数——如果在意这一点，请避免在反向代理中开启详细的访问日志。

## 致谢 {#credits}

本项目的思路来自其他创作者：

- **Mojang 服务代理**——Fallen_Breath，[Minecraft 连不上认证 Mojang 认证服务器的一种解决方案：HTTP 转发](https://blog.fallenbreath.me/zh-CN/2025/minecraft-service-proxy/)。
- **Prism Launcher 元数据改用 BMCLAPI**——SheepYhangCN，[让 Prism Launcher 使用 BMCLAPI](https://blog.sheepyuhang.top/post/prism-meta-bmclapi/)；替换规则移植自 [prism-meta-bmclapi](https://github.com/SheepYhangCN/prism-meta-bmclapi)（MIT）。
