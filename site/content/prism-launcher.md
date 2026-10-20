---
title: Prism Launcher downloads via BMCLAPI
description: Point Prism Launcher at prism-meta.yggdrasil.alisaqaq.moe so game, library and mod loader downloads come from BMCLAPI.
---

If downloading the game, its libraries and assets, or mod loaders is slow, point [Prism Launcher](https://prismlauncher.org/) at `prism-meta.yggdrasil.alisaqaq.moe`. It mirrors Prism's metadata server (`meta.prismlauncher.org/v1`) with every download URL rewritten to [BMCLAPI](https://bmclapidoc.bangbang93.com/).

## Setup {#setup}

In Prism, open *Settings → APIs* and change two fields:

| Field | Value |
| --- | --- |
| Meta URL | `https://prism-meta.yggdrasil.alisaqaq.moe/` |
| Assets server | `https://bmclapi2.bangbang93.com/assets/` |

Keep the trailing slashes. Prism downloads asset objects (sounds, languages, textures) from its own assets server setting rather than from the metadata, so the Meta URL alone does not cover them. Clear both fields to go back to Prism's defaults.

## What gets rewritten {#rewrites}

| Original host | BMCLAPI |
| --- | --- |
| `launchermeta.mojang.com`, `piston-meta.mojang.com`, `piston-data.mojang.com`, `launcher.mojang.com` | `bmclapi2.bangbang93.com` |
| `resources.download.minecraft.net` | `bmclapi2.bangbang93.com/assets` |
| `libraries.minecraft.net` | `bmclapi2.bangbang93.com/maven` |
| `meta.fabricmc.net` | `bmclapi2.bangbang93.com/fabric-meta` |
| `maven.fabricmc.net`, `maven.minecraftforge.net`, `files.minecraftforge.net/maven`, `maven.neoforged.net/releases`, `maven.quiltmc.org/repository/release` | `bmclapi2.bangbang93.com/maven` |
| `meta.quiltmc.org` | `bmclapi2.bangbang93.com/quilt-meta` |
| `dl.liteloader.com/versions` | `bmclapi2.bangbang93.com/maven` |

Only the metadata passes through this server — the downloads themselves go straight from Prism to BMCLAPI. BMCLAPI is a third-party mirror run by [bangbang93](https://github.com/bangbang93); its availability is outside this project's control.

The index files list a hash of each metadata file they point to. Those hashes no longer match the rewritten files, so they are removed; Prism skips that check when no hash is given. The hashes of the downloaded files themselves (libraries, game jars) are untouched, so downloads are still verified.

## Private instances {#private}

On an instance with a [proxy token](/#self-hosting), put it after the host:

```
https://prism-meta.yggdrasil.example.com/<secret>/
```

---

Ported from [prism-meta-bmclapi](https://github.com/SheepYhangCN/prism-meta-bmclapi) by SheepYhangCN (MIT).
