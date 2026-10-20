---
title: Mojang 服务（MC 1.16+）
description: 让 Minecraft 1.16+ 客户端、服务端和 Velocity 使用 yggdrasil.alisaqaq.moe 上 authlib 所需五项 Mojang 服务的镜像。
---

Minecraft 的 authlib 会访问五项 Mojang 服务。从 1.16 起，这些服务的地址可以通过 JVM 属性覆盖，因此只需添加启动参数即可让游戏改用镜像。每项服务都通过 `yggdrasil.alisaqaq.moe` 的独立子域名提供，请求路径原样转发。更早的版本请参阅 [authlib-injector](/zh-cn/authlib-injector/)。

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

主机名的第一个标签决定上游服务：对 `session.yggdrasil.alisaqaq.moe/session/minecraft/hasJoined` 的请求会被转发到 `sessionserver.mojang.com/session/minecraft/hasJoined`，路径与查询参数保持不变。`Authorization` 头原样透传，因此需要 Mojang Bearer 令牌的路由（玩家证书、聊天举报）不受影响。未知端点返回 JSON 404；上游失败返回 JSON 502。

## 私有实例 {#private}

对于配置了[访问令牌](/zh-cn/#self-hosting)的实例，请将令牌作为路径后缀追加到每个参数中——authlib 会在其后拼接请求路径：

```
-Dminecraft.api.auth.host=https://auth.yggdrasil.example.com/<secret>
-Dminecraft.api.account.host=https://account.yggdrasil.example.com/<secret>
-Dminecraft.api.session.host=https://session.yggdrasil.example.com/<secret>
-Dminecraft.api.services.host=https://services.yggdrasil.example.com/<secret>
-Dminecraft.api.profiles.host=https://profiles.yggdrasil.example.com/<secret>
```

Velocity：`-Dmojang.sessionserver=https://session.yggdrasil.example.com/<secret>/session/minecraft/hasJoined`。
