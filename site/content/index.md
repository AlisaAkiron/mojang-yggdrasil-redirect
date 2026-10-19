---
title: mojang-yggdrasil-redirect
description: A reverse proxy for the five Mojang services used by Minecraft's authlib — for players and servers whose network cannot reach Mojang reliably.
---

A reverse proxy for the five Mojang services used by Minecraft's authlib — for players and servers whose network cannot reach Mojang reliably. Each service is served from its own subdomain of `yggdrasil.alisaqaq.moe`, and the request path is forwarded unchanged.

:::tip
A public instance is live at `yggdrasil.alisaqaq.moe` — free for anyone to use. Add the JVM flags below and you're done; nothing else to set up. Prefer your own domain? See [Self-hosting](#self-hosting).
:::

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

The first label of the hostname selects the upstream: a request to `session.yggdrasil.alisaqaq.moe/session/minecraft/hasJoined` is forwarded to `sessionserver.mojang.com/session/minecraft/hasJoined`, with path and query untouched. Unknown endpoints get a JSON 404; upstream failures get a JSON 502.

## Self-hosting {#self-hosting}

`yggdrasil.alisaqaq.moe` is just one deployment — you can run your own instance on your own domain. Replace `yggdrasil.example.com` below with yours.

:::caution
Run it from a host whose egress IP is not blocked by Mojang. The Mojang API hosts sit behind Azure Front Door, which rejects requests from some cloud egress ranges (Cloudflare Workers, for example) — a normal VPS or residential egress works. Verify any new host first:

```sh
curl -s -o /dev/null -w '%{http_code}\n' https://sessionserver.mojang.com/session/minecraft/hasJoined   # 204 = OK
curl -s -o /dev/null -w '%{http_code}\n' https://api.minecraftservices.com/publickeys                    # 200 = OK
```
:::

Pull the published image from GHCR:

```sh
docker pull ghcr.io/alisaakiron/mojang-yggdrasil-redirect:latest
```

The service listens on `PORT` (default `8787`) and selects the upstream from the request `Host`, so run it behind a reverse proxy that terminates TLS for `*.yggdrasil.example.com`. With Traefik, container labels are enough — Traefik terminates TLS, preserves the Host header, and sets `X-Forwarded-Proto` for you:

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

The TLS certificate must cover the second-level wildcard `*.yggdrasil.example.com` (a zone's universal `*.example.com` certificate does not) — obtain it with a DNS-01 cert resolver, e.g. add `traefik.http.routers.yggdrasil.tls.certresolver=<resolver>`.

Prefer a proxy-only instance? Set `DISABLE_LANDING_PAGE=1` in the container environment and the base domain serves no docs at all — `/` returns the JSON 404 like any unknown endpoint.

For a private instance, set `PROXY_TOKEN=<secret>` (a long, URL-safe value) and append it to every flag as a path suffix — authlib adds its request paths after it:

```
-Dminecraft.api.session.host=https://session.yggdrasil.example.com/<secret>
```

The proxy validates and strips the first path segment; wrong or missing tokens get the same JSON 404 as unknown endpoints, and the `Authorization` header passes through untouched so Mojang-Bearer routes keep working. Configuring a token disables the landing page automatically. The token rides in request URLs, so keep reverse-proxy access logging off (see [Privacy](#privacy)).

## Privacy {#privacy}

Client-identifying headers (`CF-Connecting-IP`, `X-Forwarded-*`, and the like) are stripped before forwarding, so Mojang only sees this host's egress IP. The process logs only proxy errors to stdout and keeps no request log. Note that `hasJoined` request URLs contain `username` and `serverId` query parameters, so avoid verbose access logging in your reverse proxy if that matters to you.

---

Based on the approach described in [this blog post](https://blog.fallenbreath.me/zh-CN/2025/minecraft-service-proxy/).
