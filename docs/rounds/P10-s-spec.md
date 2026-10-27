# P10-s 规格：Linux `Icon=` 的名字规则与搜索路径按参考逐条核对

## 意图

`internal/apps` 把桌面条目的 `Icon=` 解析成文件路径，这件事今天**只认 hicolor + 只认
`<size>/apps` + 只认 `.png` + 不认 `Icon=name.png` 的写法 + 不认 `$XDG_DATA_HOME`**，
而参考实现是「先 pixmaps、再按主题名枚举主题、每个主题试 13 个尺寸与三种布局、最后有限
深度递归」。本仓的图标回退因此不是「参考的答案」，是「hicolor 的答案」——同一个条目在
本仓显示首字母、在参考里显示图标。

为什么现在做：P10-r 报告 §5.6 的下一轮建议（「从 `internal/apps` 的图标来源挑一条，先量三
平台各自取不到图标时的回退是否一致」）。量完的结论是：三平台的**回退**是一致的（都落到
`entry.Initials` 首字母，`launcher.go:505-524` 只有一条路），分歧在 Linux 的**解析**：
参考的搜索面比本仓宽一个数量级，而本仓那张表连 `$XDG_DATA_HOME` 都没读。

## 量出来的证据（本机实测，HEAD `5bb2483`，`internal/apps/zz_measure_test.go` 临时探针，已删）

```
Icon="firefox"        -> "/usr/share/icons/hicolor/256x256/apps/firefox.png"
Icon="firefox.png"    -> ""          ← 名字规则：带扩展名一律解析不到
Icon="Alacritty"      -> ""          ← 本机只有 /usr/share/pixmaps/Alacritty.svg
Icon="btop.png"       -> ""
iconSizes = 256x256 128x128 64x64 48x48 32x32 scalable        （参考 13 项）
iconDirs  = ~/.local/share/icons/hicolor, ~/.local/share/icons, ~/.local/share/pixmaps,
            /usr/share/icons/hicolor, /usr/local/share/icons/hicolor, /usr/share/icons,
            /usr/local/share/icons, /usr/share/pixmaps
```

四条能直接说清代价的事实：

1. **`Icon=name.png` 解析不到**。本仓 `desktopIconFile`（`internal/apps/icons.go:171-196`）
   直接 `base+".png"`，`Icon=firefox.png` 会去找 `firefox.png.png`。参考先取
   `Path::file_stem()`（`main:src-tauri/src/commands/apps/linux.rs:603-614`）。
2. **只认 hicolor**。`iconDirs()`（`icons.go:203-221`）把 `hicolor` 直接写进目录名，
   于是 `/usr/share/icons/Papirus/48x48/apps/x.png`、`Adwaita`、`breeze` 一律找不到；
   参考枚举 base 下的**主题目录**（`themes_in`，`linux.rs:659-683`），偏好序
   `hicolor, Adwaita, breeze, Papirus, gnome, default` 之后还有其余主题。
3. **尺寸表少了七档、且 256 在 512 前**。本机 `/usr/share/icons/hicolor/512x512/apps/qemu.png`
   与 `256x256/apps/qemu.png` 都在，实测本仓取 **256**，参考取 **512**
   （`ICON_SIZES`，`linux.rs:21-24`）。缺的 `192x192/96x96/72x72/36x36/24x24/16x16`
   是「只在那一档有图」的条目唯一的图。
4. **不读 `$XDG_DATA_HOME`，也没有 flatpak 与 `~/.icons`**。参考的 `icon_base_dirs`
   （`linux.rs:637-656`）是 `~/.icons`、`$XDG_DATA_HOME/icons`、
   `$XDG_DATA_HOME/flatpak/exports/share/icons`、`/usr/local/share/icons`、
   `/usr/share/icons`、`/var/lib/flatpak/exports/share/icons`、`$XDG_DATA_DIRS/*/icons`
   （去重）；pixmap 面是 `$XDG_DATA_HOME/pixmaps` 然后 `/usr/share/pixmaps`
   （`pixmap_dirs`，`linux.rs:658-665`），且 **pixmaps 在主题之前**（`resolve_icon_uncached`
   `linux.rs:603-635` 的顺序）。本仓顺序相反，且 `$XDG_DATA_HOME` 整个没读——
   `internal/xdg`（P9-s）早就有 `DataHome()`/`DataDirs()`，这里是第二份硬编码。

顺带量清的两条**不能照抄**的参考行为：

- **`file_stem` 会截断带点的图标名**。本机 `/usr/share/icons/hicolor/*/apps/org.gnome.gedit.png`、
  `org.cachyos.KernelManager.png`、`org.cachyos.scx-manager.png` 都是**带点的真文件名**，
  而 `/usr/share/applications/*.desktop` 里 `Icon=org.gnome.Calculator` 这类值有二十多个。
  参考的 `file_stem` 会把 `org.cachyos.KernelManager` 截成 `org.cachyos`，永远找不到。
  本仓今天**不截断**，这些条目是有图标的；照抄 = 把它们弄坏。所以本仓只剥**已知图标扩展名**
  （`png`），带点的名字原样保留——这是本轮唯一的名字规则分歧，必须钉住。
- **参考把 svg/xpm 也算候选**（`ICON_EXTENSIONS`，`linux.rs:27`）。本仓的渲染面是
  `ui.DecodeBitmap`（`internal/launcher/launcher.go:506`）＝只吃 PNG，`readIcon` 的
  `isPNG` 会把 svg 判死（`icons.go:131`）。所以搜索面只收 `.png`：返回一个解码不了的文件
  比返回空更糟（它会把同一尺寸下更小的 png 挡掉）。Alacritty（本机只有 svg）继续显示首字母，
  这是**能力差**，写进报告与 `.pending-bugs.md`，不在本轮伪装成已解决。

## 落地项（动手前已按 HEAD `5bb2483` 核对行号）

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/apps/icons.go:171-181` `desktopIconFile` 的名字规则 | `base := filepath.Base(name)` 直接拼 `.png` | 先剥**已知图标扩展名**（`.png`，大小写不敏感）得到 stem；`Icon=foo.png` 与 `Icon=foo` 等价。带点但不是图标扩展名的名字（`org.gnome.gedit`）**不截断** |
| `internal/apps/icons.go:182-195` 搜索顺序 | 先 `iconDirs()`（写死 hicolor 等），每项 `<dir>/<size>/apps` 再 `<dir>/x.png` | ①pixmap 面：`$XDG_DATA_HOME/pixmaps`、`/usr/share/pixmaps`，只看 `<dir>/<name>.png`；②主题面：`iconBaseDirs()` 里每个 base 枚举主题（偏好序在前），每个主题 `findInTheme` |
| `internal/apps/icons.go:198-200` `iconSizes` | 6 项，`256x256` 第一 | 参考 `ICON_SIZES` 的 13 项原序（`512x512 … scalable … 16x16`） |
| `internal/apps/icons.go:203-221` `iconDirs()` | 8 项硬编码，`hicolor` 写进目录名 | 换 `iconBaseDirs()`：`~/.icons`、`$XDG_DATA_HOME/icons`、`$XDG_DATA_HOME/flatpak/exports/share/icons`、`/usr/local/share/icons`、`/usr/share/icons`、`/var/lib/flatpak/exports/share/icons`、`$XDG_DATA_DIRS/*/icons`，去重（参考顺序）；`$XDG_DATA_HOME`/`$XDG_DATA_DIRS` 走 `internal/xdg` 的 `DataHome()`/`DataDirs()`，不再有第二份 |
| 新增 `findInTheme(themeDir, name)` | 无（只有 `<dir>/<size>/apps`） | 每个尺寸试 `theme/<size>/apps`、`theme/apps/<size>`、`theme/apps/<数字>`（breeze 式）；再 `theme/apps/<name>.png`、`theme/<name>.png`；最后 `searchRecursive` 深度 ≤3（参考 `search_recursive`，`linux.rs:726-760`），只收 `.png` |
| 新增 `themesIn(base)` | 无 | 偏好序 `hicolor, Adwaita, breeze, Papirus, gnome, default`（存在的目录才进表），随后 base 下其余**目录**按名字排序（参考用 `read_dir` 的读序；本仓排序是为了跨机器可复现，见报告「判不准项」） |
| 新增 `pixmapDirs()` | 无 | `$XDG_DATA_HOME/pixmaps`、`/usr/share/pixmaps` |
| 新 `internal/apps/iconsearch_test.go` | 无 | 见下 |

## 新 pin 清单（`internal/apps/iconsearch_test.go`）

全部用 `t.Setenv("HOME"/"XDG_DATA_HOME"/"XDG_DATA_DIRS")` + `t.TempDir()` 造主题树，
名字一律带 `p10s-` 前缀，保证真实系统目录不可能命中（测试与本机内容解耦）。

1. `TestTheIconNameLosesOnlyAKnownExtension`：`Icon=p10s-a.png` 命中 `p10s-a.png`；
   `Icon=p10s-b` 命中同一文件；`Icon=org.p10s.kept` **不**被截成 `org.p10s`（带点的名字
   原样保留，参考 `file_stem` 会截断——本轮故意不同，注释写明理由与真文件名证据）。
2. `TestPixmapsComeBeforeThemes`：同一名字在 `$XDG_DATA_HOME/pixmaps` 与 hicolor 主题里
   都有 → 取 pixmap（参考顺序）；pixmap 面同时读 `$XDG_DATA_HOME/pixmaps` 与 `/usr/share/pixmaps`
   （后者用 `/usr/share/pixmaps` 自身断言目录表，不造文件）。
3. `TestEveryThemeUnderABaseIsSearched`：`<base>/Papirus/48x48/apps/x.png` 命中（本仓旧形状
   只会在 `<base>/48x48/apps` 找）；偏好序：同一名字在 `Adwaita` 与 `Papirus` 都有 → 取
   `Adwaita`。
4. `TestTheThreeThemeLayouts`：`<theme>/48x48/apps/x.png`、`<theme>/apps/48x48/x.png`、
   `<theme>/apps/48/x.png` 三种各命中一次。
5. `TestTheLargestSizeWins`：`512x512` 与 `256x256` 都在 → 取 512（本机 `qemu` 的实测）；
   `iconSizes` 的 13 项真值表逐项断言。
6. `TestTheIconBaseDirsAreTheReferences`：`~/.icons`、`$XDG_DATA_HOME/icons`、
   `$XDG_DATA_HOME/flatpak/exports/share/icons`、`/usr/local/share/icons`、`/usr/share/icons`、
   `/var/lib/flatpak/exports/share/icons`、`$XDG_DATA_DIRS` 的两项 `*/icons`，顺序与去重
   （`$XDG_DATA_DIRS` 里放一个已在表中的目录，只出现一次）。
7. `TestAnUnusualLayoutIsFoundThreeDeep`：`<theme>/a/b/c/x.png` 命中，`<theme>/a/b/c/d/x.png`
   不命中（参考深度上限 3）。
8. `TestOnlyPNGIconsAreReturned`：只有 `x.svg` 时返回 `""`（Alacritty 实测：本机 pixmaps
   只有 `Alacritty.svg`）；同一名字 `x.svg` + 更小尺寸的 `x.png` 时返回那张 png
   （svg 不许把 png 挡掉）。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**一行不动**；`Icon`/`IconPending`
的异步与缓存语义（`icons.go:59-118`）、`readIcon` 的解码规则（`icons.go:122-137`）、
`ScanLinux`/`scanLinuxRoot` 的走法、macOS 与 Windows 的取图路径（`darwin.go:69-70`、
`icons_windows.go`）**一字不动**。本轮只改「`Icon=` 这个名字到文件路径」的搜索面，
无墨无面（不出图）。

## 门槛与 PASS 基线（HEAD `5bb2483` 实测）

```
gofmt -l cmd internal                       → 空
go vet ./...                                → 净
go test -count=1 ./...                      → 全绿
--- PASS 计数                                → 2138（基线）
--- SKIP 计数                                → 15（=）
测试函数（^func Test）                        → 1740（+8 → 1748）
GOOS=linux/windows/darwin go build ./...     → 三条都过
```

## mutation 计划（每条说明会让哪个 pin 变红）

1. 删掉 `findInTheme` 的 `theme/apps/<数字>` 布局 → `TestTheThreeThemeLayouts` 红。
2. `iconSizes` 的 `512x512` 与 `256x256` 换位 → `TestTheLargestSizeWins` 红。
3. 名字规则换回「不剥扩展名」→ `TestTheIconNameLosesOnlyAKnownExtension` 红。
4. `pixmapDirs`/主题面的顺序对调 → `TestPixmapsComeBeforeThemes` 红。
