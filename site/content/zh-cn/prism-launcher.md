---
title: Prism Launcher 使用 BMCLAPI 下载
description: 让 Prism Launcher 使用 prism-meta.yggdrasil.alisaqaq.moe，从 BMCLAPI 下载游戏、依赖库和模组加载器。
---

如果下载游戏本体、依赖库与资源文件或模组加载器很慢，可以让 [Prism Launcher](https://prismlauncher.org/) 使用 `prism-meta.yggdrasil.alisaqaq.moe`。它镜像 Prism 的元数据服务器（`meta.prismlauncher.org/v1`），并把其中的下载地址全部替换为 [BMCLAPI](https://bmclapidoc.bangbang93.com/)。

## 配置 {#setup}

在 Prism 中打开 *设置 → API*，修改以下两项：

| 设置项 | 值 |
| --- | --- |
| 元数据地址（Meta URL） | `https://prism-meta.yggdrasil.alisaqaq.moe/` |
| 资源服务器（Assets server） | `https://bmclapi2.bangbang93.com/assets/` |

请保留末尾的斜杠。Prism 下载资源文件（声音、语言、材质）时使用的是自己的资源服务器设置，而不是元数据中的地址，因此只改元数据地址无法覆盖这部分。清空这两项即可恢复 Prism 的默认设置。

## 替换规则 {#rewrites}

| 原始地址 | BMCLAPI |
| --- | --- |
| `launchermeta.mojang.com`、`piston-meta.mojang.com`、`piston-data.mojang.com`、`launcher.mojang.com` | `bmclapi2.bangbang93.com` |
| `resources.download.minecraft.net` | `bmclapi2.bangbang93.com/assets` |
| `libraries.minecraft.net` | `bmclapi2.bangbang93.com/maven` |
| `meta.fabricmc.net` | `bmclapi2.bangbang93.com/fabric-meta` |
| `maven.fabricmc.net`、`maven.minecraftforge.net`、`files.minecraftforge.net/maven`、`maven.neoforged.net/releases`、`maven.quiltmc.org/repository/release` | `bmclapi2.bangbang93.com/maven` |
| `meta.quiltmc.org` | `bmclapi2.bangbang93.com/quilt-meta` |
| `dl.liteloader.com/versions` | `bmclapi2.bangbang93.com/maven` |

只有元数据会经过本服务——实际下载由 Prism 直接连接 BMCLAPI。BMCLAPI 是由 [bangbang93](https://github.com/bangbang93) 运营的第三方镜像，其可用性不受本项目控制。

索引文件中记录了其所指向的每个元数据文件的哈希。改写后的文件与这些哈希不再一致，因此它们会被移除；没有哈希时 Prism 会跳过这项校验。下载文件本身（依赖库、游戏 jar）的哈希保持不变，因此下载内容仍会被校验。

## 私有实例 {#private}

对于配置了[访问令牌](/zh-cn/#self-hosting)的实例，请把令牌放在主机名之后：

```
https://prism-meta.yggdrasil.example.com/<secret>/
```

## 致谢 {#credits}

- **Prism Launcher 元数据改用 BMCLAPI**——SheepYhangCN，[让 Prism Launcher 使用 BMCLAPI](https://blog.sheepyuhang.top/post/prism-meta-bmclapi/)；替换规则移植自 [prism-meta-bmclapi](https://github.com/SheepYhangCN/prism-meta-bmclapi)（MIT）。
