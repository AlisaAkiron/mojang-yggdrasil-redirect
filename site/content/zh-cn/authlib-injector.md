---
title: 旧版本：使用 authlib-injector
description: 通过 authlib-injector 在 Minecraft 1.7.10 – 1.15 上使用 yggdrasil.alisaqaq.moe 的 Mojang 镜像，照常使用微软账号与原版皮肤。
---

1.16 之前的 authlib 把 Mojang 的地址写死在代码里，会忽略 [Mojang 服务指南](/zh-cn/yggdrasil/)中的 `minecraft.api.*` 参数。这些版本（例如 GT New Horizons 等 1.7.10 整合包）请使用 [authlib-injector](https://github.com/yushijinhun/authlib-injector)，并以 `authlib-injector.yggdrasil.alisaqaq.moe` 作为 API 地址。它同样适用于 1.16+，如果你希望所有版本使用同一套配置也可以用它。

## 配置 {#setup}

从 authlib-injector 的 [Releases](https://github.com/yushijinhun/authlib-injector/releases) 下载 `authlib-injector.jar`，然后在服务端和/或客户端的 `-jar` 之前加上：

```
-javaagent:authlib-injector.jar=https://authlib-injector.yggdrasil.alisaqaq.moe
```

如果 jar 不在工作目录中，请填写它的实际路径。玩家照常使用自己的微软账号，登录方式没有任何变化。

## 工作原理 {#how-it-works}

authlib-injector 会把发往 `https://<Mojang 主机>/<路径>` 的请求改写为 `<API 地址>/<前缀>/<路径>`。该子域名使用同样的结构，去掉前缀后把剩余部分转发给 Mojang：

| 前缀 | 上游服务 |
| --- | --- |
| `/authserver` | `authserver.mojang.com` |
| `/sessionserver` | `sessionserver.mojang.com` |
| `/api` | `api.mojang.com` |
| `/minecraftservices` | `api.minecraftservices.com` |

API 地址本身（`/`）返回 authlib-injector 启动时读取的元数据，其中提供的是 Mojang 自己的会话签名公钥，因此皮肤和正版验证与原版完全一致。

## 私有实例 {#private}

对于配置了[访问令牌](/zh-cn/#self-hosting)的实例，请把令牌附加在 API 地址之后：

```
-javaagent:authlib-injector.jar=https://authlib-injector.yggdrasil.example.com/<secret>
```
