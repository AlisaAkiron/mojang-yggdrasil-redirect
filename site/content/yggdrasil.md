---
title: Mojang services (MC 1.16+)
description: Point Minecraft 1.16+ clients, servers and Velocity at the yggdrasil.alisaqaq.moe mirrors of the five Mojang services used by authlib.
---

Minecraft's authlib talks to five Mojang services. Since 1.16 their hosts can be overridden with JVM properties, so pointing the game at the mirrors needs nothing but launch flags. Each service is served from its own subdomain of `yggdrasil.alisaqaq.moe`, and the request path is forwarded unchanged. For older versions, see [authlib-injector](/authlib-injector/).

## Quick start {#quick-start}

Add these JVM flags before `-jar` (Minecraft 1.16+, client or server):

```
-Dminecraft.api.auth.host=https://auth.yggdrasil.alisaqaq.moe
-Dminecraft.api.account.host=https://account.yggdrasil.alisaqaq.moe
-Dminecraft.api.session.host=https://session.yggdrasil.alisaqaq.moe
-Dminecraft.api.services.host=https://services.yggdrasil.alisaqaq.moe
-Dminecraft.api.profiles.host=https://profiles.yggdrasil.alisaqaq.moe
```

As a one-liner:

```
-Dminecraft.api.auth.host=https://auth.yggdrasil.alisaqaq.moe -Dminecraft.api.account.host=https://account.yggdrasil.alisaqaq.moe -Dminecraft.api.session.host=https://session.yggdrasil.alisaqaq.moe -Dminecraft.api.services.host=https://services.yggdrasil.alisaqaq.moe -Dminecraft.api.profiles.host=https://profiles.yggdrasil.alisaqaq.moe
```

A Velocity proxy only needs:

```
-Dmojang.sessionserver=https://session.yggdrasil.alisaqaq.moe/session/minecraft/hasJoined
```

## Endpoints {#endpoints}

| Mirror | Upstream | JVM property | Notes |
| --- | --- | --- | --- |
| `auth.yggdrasil.alisaqaq.moe` | `authserver.mojang.com` | `minecraft.api.auth.host` | Legacy login; unused since MC 1.20.2 |
| `account.yggdrasil.alisaqaq.moe` | `api.mojang.com` | `minecraft.api.account.host` | Player info; unused since MC 1.20.3 |
| `session.yggdrasil.alisaqaq.moe` | `sessionserver.mojang.com` | `minecraft.api.session.host` | Online-mode join verification |
| `services.yggdrasil.alisaqaq.moe` | `api.minecraftservices.com` | `minecraft.api.services.host` | Chat reports etc.; since MC 1.16.4 |
| `profiles.yggdrasil.alisaqaq.moe` | `api.mojang.com` | `minecraft.api.profiles.host` | Player info; since authlib 7.0.61 |

## How it works {#how-it-works}

The first label of the hostname selects the upstream: a request to `session.yggdrasil.alisaqaq.moe/session/minecraft/hasJoined` is forwarded to `sessionserver.mojang.com/session/minecraft/hasJoined`, with path and query untouched. The `Authorization` header passes through, so routes that need a Mojang Bearer token (player certificates, chat reports) keep working. Unknown endpoints get a JSON 404; upstream failures get a JSON 502.

## Private instances {#private}

On an instance with a [proxy token](/#self-hosting), append the token to every flag as a path suffix — authlib adds its request paths after it:

```
-Dminecraft.api.auth.host=https://auth.yggdrasil.example.com/<secret>
-Dminecraft.api.account.host=https://account.yggdrasil.example.com/<secret>
-Dminecraft.api.session.host=https://session.yggdrasil.example.com/<secret>
-Dminecraft.api.services.host=https://services.yggdrasil.example.com/<secret>
-Dminecraft.api.profiles.host=https://profiles.yggdrasil.example.com/<secret>
```

For Velocity: `-Dmojang.sessionserver=https://session.yggdrasil.example.com/<secret>/session/minecraft/hasJoined`.
