---
title: mojang-yggdrasil-redirect
description: Mirrors of the Mojang services used by Minecraft's authlib, plus Prism Launcher metadata with BMCLAPI downloads — for players and servers whose network cannot reach Mojang reliably.
---

Mirrors for players and servers whose network cannot reach Mojang reliably: the five Mojang services used by Minecraft's authlib, an [authlib-injector](https://github.com/yushijinhun/authlib-injector) endpoint for older versions, and Prism Launcher metadata that downloads from BMCLAPI. Each is served from its own subdomain of `yggdrasil.alisaqaq.moe`.

:::tip
A public instance is live at `yggdrasil.alisaqaq.moe` — free for anyone to use, nothing to set up. Pick a guide below. Prefer your own domain? See [Self-hosting](#self-hosting).
:::

## Guides {#guides}

| Guide | Use it for | Subdomain |
| --- | --- | --- |
| [Mojang services](/yggdrasil/) | Minecraft 1.16+ clients and servers, Velocity | `auth`, `account`, `session`, `services`, `profiles` |
| [authlib-injector](/authlib-injector/) | Minecraft 1.7.10 – 1.15 (e.g. GT New Horizons) | `authlib-injector` |
| [Prism Launcher](/prism-launcher/) | Faster game, library and mod loader downloads in Prism | `prism-meta` |

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
    environment:
      - "ENABLE_MODULE=yggdrasil,authlib-injector,prism-meta"
    labels:
      - "traefik.enable=true"
      - "traefik.http.routers.yggdrasil.rule=Host(`yggdrasil.example.com`) || Host(`*.yggdrasil.example.com`)"
      - "traefik.http.routers.yggdrasil.entrypoints=websecure"
      - "traefik.http.routers.yggdrasil.tls=true"
      - "traefik.http.services.yggdrasil.loadbalancer.server.port=8787"
```

The TLS certificate must cover the second-level wildcard `*.yggdrasil.example.com` (a zone's universal `*.example.com` certificate does not) — obtain it with a DNS-01 cert resolver, e.g. add `traefik.http.routers.yggdrasil.tls.certresolver=<resolver>`.

Every feature is a module, and **all modules are disabled by default** — list the ones you want, comma-separated, in `ENABLE_MODULE`:

| Module | Subdomains |
| --- | --- |
| [`yggdrasil`](/yggdrasil/) | `auth`, `account`, `session`, `services`, `profiles` |
| [`authlib-injector`](/authlib-injector/) | `authlib-injector` |
| [`prism-meta`](/prism-launcher/) | `prism-meta` |

A disabled module's subdomains return the JSON 404, and the service refuses to start with no modules enabled or an unknown module name.

Prefer a proxy-only instance? Set `DISABLE_LANDING_PAGE=1` in the container environment and the base domain serves no docs at all — `/` returns the JSON 404 like any unknown endpoint.

For a private instance, set `PROXY_TOKEN=<secret>` (a long, URL-safe value). Every request must then carry the token as the first path segment — each guide shows where it goes. The proxy validates and strips it; wrong or missing tokens get the same JSON 404 as unknown endpoints. Configuring a token disables the landing page automatically. The token rides in request URLs, so keep reverse-proxy access logging off (see [Privacy](#privacy)).

## Privacy {#privacy}

Client-identifying headers (`CF-Connecting-IP`, `X-Forwarded-*`, and the like) are stripped before forwarding, so Mojang only sees this host's egress IP. The process logs only proxy errors to stdout and keeps no request log. Note that `hasJoined` request URLs contain `username` and `serverId` query parameters, so avoid verbose access logging in your reverse proxy if that matters to you.

---

Based on the approach described in [this blog post](https://blog.fallenbreath.me/zh-CN/2025/minecraft-service-proxy/).
