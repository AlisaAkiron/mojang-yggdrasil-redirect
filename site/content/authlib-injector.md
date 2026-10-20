---
title: Older versions via authlib-injector
description: Use the yggdrasil.alisaqaq.moe Mojang mirrors on Minecraft 1.7.10 – 1.15 through authlib-injector, with normal Microsoft accounts and vanilla skins.
---

Before 1.16, authlib hardcodes the Mojang URLs and ignores the `minecraft.api.*` flags from the [Mojang services guide](/yggdrasil/). For those versions (e.g. 1.7.10 modpacks such as GT New Horizons), use [authlib-injector](https://github.com/yushijinhun/authlib-injector) with `authlib-injector.yggdrasil.alisaqaq.moe` as its API root. It works on 1.16+ too, if you prefer one setup for every version.

## Setup {#setup}

Download `authlib-injector.jar` from its [releases](https://github.com/yushijinhun/authlib-injector/releases) and add this before `-jar`, on the server and/or the client:

```
-javaagent:authlib-injector.jar=https://authlib-injector.yggdrasil.alisaqaq.moe
```

Use the jar's actual path if it is not in the working directory. Players keep their normal Microsoft accounts — nothing changes in how they log in.

## How it works {#how-it-works}

authlib-injector rewrites every request to `https://<mojang host>/<path>` into `<API root>/<prefix>/<path>`. This subdomain speaks that layout, strips the prefix, and forwards the rest to Mojang:

| Prefix | Upstream |
| --- | --- |
| `/authserver` | `authserver.mojang.com` |
| `/sessionserver` | `sessionserver.mojang.com` |
| `/api` | `api.mojang.com` |
| `/minecraftservices` | `api.minecraftservices.com` |

The API root itself (`/`) returns the metadata authlib-injector reads at startup. It hands out Mojang's own session signing key, so skins and online-mode verification work exactly as in vanilla.

## Private instances {#private}

On an instance with a [proxy token](/#self-hosting), append it to the API root:

```
-javaagent:authlib-injector.jar=https://authlib-injector.yggdrasil.example.com/<secret>
```
