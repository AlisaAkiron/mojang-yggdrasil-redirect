# mojang-yggdrasil-redirect

English | [简体中文](README.zh-CN.md)

A small Go reverse proxy for the five Mojang services used
by Minecraft's authlib, for players/servers whose network cannot reach Mojang reliably.

Each service is served from its own subdomain; the first hostname label selects the
upstream and the path is forwarded unchanged.

> [!TIP]
> **A public instance is live at [`yggdrasil.alisaqaq.moe`](https://yggdrasil.alisaqaq.moe/)** — hosted on
> my own server and free for anyone to use. Visit it for the endpoint list and ready-to-copy
> setup instructions (JVM flags, authlib-injector, Prism Launcher). Prefer to run your own?
> See [Self-hosting](#self-hosting-docker) to deploy with your own domain.

## Development

Requires Go 1.27+ (`nix develop` / direnv provides it).

```sh
go generate ./site  # Markdown docs -> site/dist (embedded into the api binary)
go run ./cmd/api    # http://localhost:8787
go test ./...       # unit tests
go build ./cmd/api  # single static binary, docs included
```

`HOST` and `PORT` set the listen address (default `0.0.0.0:8787`).

The docs are Markdown under `site/content/`, with a `title`/`description`
frontmatter block. Besides GitHub-style tables, the generator supports explicit heading
anchors (`## 自行部署 {#self-hosting}`, so every locale shares the same `#ids`) and
callouts:

```md
:::tip[Optional title]
Markdown content
:::
```

Re-run `go generate ./site` after editing, then restart the api. `site/dist` is
embedded at compile time, so a binary built without generating serves no docs. The
public domain is written literally in the content; at runtime the server rewrites it
to the serving domain, so self-hosted instances stay correct. Adding a language =
adding `site/content/<dir>/index.md` and a `locales` entry in `site/config.json`; pages
with the same name in each locale link to each other.

Test a subdomain locally via the Host header:

```sh
curl -s http://localhost:8787/session/minecraft/hasJoined -H 'Host: session.yggdrasil.localhost'   # 204
```

## Self-hosting (Docker)

`yggdrasil.alisaqaq.moe` is just my deployment — you can run your own instance on your own
domain. Replace `yggdrasil.example.com` below with yours.

> [!IMPORTANT]
> **Run it from a host whose egress IP is not blocked by Mojang.** The Mojang API hosts
> sit behind Azure Front Door, which returns `403 The request is blocked` to requests
> originating from Cloudflare's Workers egress IPs — that is why this is a plain
> service you self-host (VPS / homelab), **not** a Cloudflare Worker. A normal
> VPS/residential egress works. Verify any new host first:
> ```sh
> curl -s -o /dev/null -w '%{http_code}\n' https://sessionserver.mojang.com/session/minecraft/hasJoined   # 204 = OK
> curl -s -o /dev/null -w '%{http_code}\n' https://api.minecraftservices.com/publickeys                    # 200 = OK
> ```

The service listens on `PORT` (default `8787`) and selects the upstream from the request
`Host`, so run it behind a reverse proxy that terminates TLS for `*.<your-domain>`.

Pull the published image from GHCR:

```sh
docker pull ghcr.io/alisaakiron/mojang-yggdrasil-redirect:latest
```

Run it behind Traefik with container labels — Traefik terminates TLS, preserves the Host
header, and sets `X-Forwarded-Proto` for you:

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

The TLS cert must cover the second-level wildcard `*.yggdrasil.example.com` (a zone's universal
`*.example.com` cert does not) — obtain it with a DNS-01 cert resolver, e.g. add
`traefik.http.routers.yggdrasil.tls.certresolver=<resolver>`.

Every feature is a module, and **all modules are disabled by default** — list the ones you want, comma-separated, in `ENABLE_MODULE`:

| Module | Subdomains |
| --- | --- |
| `yggdrasil` | `auth`, `account`, `session`, `services`, `profiles` |
| `authlib-injector` | `authlib-injector` |
| `prism-meta` | `prism-meta` |

A disabled module's subdomains return the JSON 404, and the service refuses to start with no modules enabled or an unknown module name.

Prefer a proxy-only instance? Set `DISABLE_LANDING_PAGE=1` in the container environment
and the base domain serves no docs at all — `/` returns the JSON 404 like any unknown
endpoint.

For a private instance, set `PROXY_TOKEN=<secret>` (pick a long, URL-safe value) and
include it as a path suffix in every flag — authlib appends its request paths after it:

```
-Dminecraft.api.auth.host=https://auth.yggdrasil.example.com/<secret>
-Dminecraft.api.account.host=https://account.yggdrasil.example.com/<secret>
-Dminecraft.api.session.host=https://session.yggdrasil.example.com/<secret>
-Dminecraft.api.services.host=https://services.yggdrasil.example.com/<secret>
-Dminecraft.api.profiles.host=https://profiles.yggdrasil.example.com/<secret>
```

The proxy validates the first path segment (timing-safe), strips it, and forwards the
rest; wrong or missing tokens get the same JSON 404 as unknown endpoints. The
`Authorization` header passes through untouched, so Mojang-Bearer routes (player
certificates, chat reports) keep working. Configuring a token disables the landing
page automatically. Since the token rides in request URLs, keep reverse-proxy access
logging off (see [Privacy](#privacy)).

## Privacy

Client-identifying headers (`CF-Connecting-IP`, `X-Forwarded-*`, etc.) are stripped
before forwarding, so Mojang only sees this host's egress IP. The process logs only
proxy errors (a JSON `proxy_error` line) to stdout; it keeps no request log. Note that
`hasJoined` request URLs contain `username`/`serverId` query parameters, so avoid adding
verbose access logging in your reverse proxy if that matters to you.

## Credits

The ideas behind this project come from other creators:

- **Mojang services proxy** (`yggdrasil` module) — Fallen_Breath,
  [Minecraft 连不上认证 Mojang 认证服务器的一种解决方案：HTTP 转发](https://blog.fallenbreath.me/zh-CN/2025/minecraft-service-proxy/)
  (in Chinese).
- **Prism Launcher metadata via BMCLAPI** (`prism-meta` module) — SheepYhangCN,
  [让 Prism Launcher 使用 BMCLAPI](https://blog.sheepyuhang.top/post/prism-meta-bmclapi/) (in Chinese);
  the rewrite rules are ported from [prism-meta-bmclapi](https://github.com/SheepYhangCN/prism-meta-bmclapi) (MIT).
