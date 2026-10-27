# P10-s 报告：Linux `Icon=` 的名字规则与搜索路径按参考逐条核对

规格 `docs/rounds/P10-s-spec.md`。方向：P10-r 报告 §5.6 的下一轮建议（「从 `internal/apps`
的图标来源挑一条，先量三平台各自取不到图标时的回退是否一致」）。量完的结论写在 §0：
**回退一致，解析不一致**——三平台取不到图标时都落到首字母（`internal/launcher/launcher.go:505-524`
只有一条路），分歧全在 Linux 的 `Icon=` 解析面。

## §0 量出来的四件事（HEAD `5bb2483`，本机实测）

用临时探针（`internal/apps/zz_measure_test.go`，跑完即删）量的原始输出：

```
Icon="firefox"        -> "/usr/share/icons/hicolor/256x256/apps/firefox.png"
Icon="firefox.png"    -> ""
Icon="Alacritty"      -> ""        （本机只有 /usr/share/pixmaps/Alacritty.svg）
iconSizes = 256x256 128x128 64x64 48x48 32x32 scalable
ScanLinux("/usr/share/applications") → entries=48 withIconPath=17 elapsed=191ms
```

| # | 分歧 | 参考 | 本仓（改前） | 代价 |
|---|---|---|---|---|
| 1 | 名字规则 | `Path::file_stem`（`linux.rs:603-614`） | 直接 `base+".png"` | `Icon=firefox.png` 去找 `firefox.png.png`，永远空 |
| 2 | 主题枚举 | `themes_in`（`linux.rs:667-683`），偏好序 + 其余主题 | 目录名里写死 `hicolor`（`icons.go:213-219`） | Papirus / breeze / Adwaita 里的图标一律找不到 |
| 3 | 尺寸表 | 13 项（`ICON_SIZES`，`linux.rs:21-24`） | 6 项，`256` 在 `512` 前 | 本机 `qemu` 同时有 512 与 256，取 256 |
| 4 | 目录表 | `~/.icons` + `$XDG_DATA_HOME/icons` + flatpak exports + `$XDG_DATA_DIRS/*/icons`（`linux.rs:637-656`），pixmaps **先于**主题（`linux.rs:603-635`） | 硬编码 8 项，`~/.local/share` 写死，pixmaps 在最后 | 自定义 `$XDG_DATA_HOME`、flatpak 应用、`/usr/share/pixmaps` 里的图标都看不到 |

顺带量清两条**不能照抄**的参考行为：

- **`file_stem` 会截断带点的图标名**。`/usr/share/icons/hicolor/*/apps/` 里真有
  `org.gnome.gedit.png`、`org.cachyos.KernelManager.png`、`org.cachyos.scx-manager.png`，
  而 `/usr/share/applications` 有二十多个 `Icon=org.gnome.X`。参考的 `file_stem` 会把
  `org.cachyos.KernelManager` 截成 `org.cachyos` → 找不到。本仓只剥**已知图标扩展名**
  （`.png/.svg/.xpm`），带点的名字原样保留。这是本轮唯一的名字规则分歧。
- **参考把 svg/xpm 也算候选**（`ICON_EXTENSIONS`）。本仓的渲染面是 `ui.DecodeBitmap`
  （`launcher.go:506`），`readIcon` 的 `isPNG` 会把 svg 判死。返回一个解码不了的文件比返回
  空更糟（它会把同尺寸更小的 png 挡掉），所以搜索面只收 `.png`。

## §1 落地

`internal/apps/icons.go`（产品代码只改这一个文件）：

- `desktopIconFile`（`icons.go:192-227`）：先 pixmaps、再 base→主题→`findInTheme`。
- `iconName` + `iconNameExtensions`（`icons.go:236-253`）：`Icon=foo.png|.svg|.xpm` 与
  `Icon=foo` 等价；带点但不是图标扩展名的名字不截断。
- `iconSizes`（`icons.go:255-263`）：参考 13 项原序（`scalable` 在 `48x48` 之后）。
- `preferredThemes`（`icons.go:265-267`）：`hicolor, Adwaita, breeze, Papirus, gnome, default`。
- `pixmapDirs` / `iconBaseDirs`（`icons.go:269-311`）：`$XDG_DATA_HOME` 与 `$XDG_DATA_DIRS`
  走 `internal/xdg` 的 `DataHome()`/`DataDirs()`——P9-s 之后这里不该有第二份答案。
- `themesIn`（`icons.go:313-341`）：偏好序在前，base 下其余**目录**按名字排序在后。
- `findInTheme`（`icons.go:343-374`）：每尺寸三种布局（`<size>/apps`、`apps/<size>`、
  `apps/<数字>`，breeze 式），再 `apps/`、主题根，最后 `searchIconDir`。
- `searchIconDir` + `indexIconDir` + `iconIndexes`（`icons.go:376-437`）：深度 ≤3 的兜底
  走法，**每个主题目录只走一次**并建 name→path 表；`resetIconCaches` 给测试用。

关于那张索引表：无缓存版实测把 `ScanLinux` 从 **191ms 抬到 686ms**（48 个条目，26 个未命中，
每个未命中要走 ~6 个主题 × 13 尺寸 × 3 布局 + 深度 3 的目录树）。参考用「按 `Icon=` 值
memo」摊掉这笔开销（`icon_cache`，`linux.rs:581`）；本仓按**主题目录**建表，第一次未命中
付走法，之后是 map 查找。实测 **30ms**（比改前还快，因为不再每个名字重复 stat 同一批目录）。
索引与参考的 memo 一样不失效：运行期新装的主题下次启动才出现。

## §2 本机真实收益（同一台机器，改前 17 个条目有图标，改后 21 个）

| 条目 | `Icon=` | 改前 | 改后 |
|---|---|---|---|
| gedit | `org.gnome.gedit` | 空 | `hicolor/256x256/apps/org.gnome.gedit.png` |
| kitty | `kitty` | `hicolor/256x256/apps/kitty.png` | `/usr/share/pixmaps/kitty.png`（pixmaps 先于主题，两张都是 256×256） |
| Avahi SSH/VNC/Zeroconf | `network-wired` | 空 | `AdwaitaLegacy/16x16/legacy/network-wired.png`（主题枚举 + 兜底走法） |
| X11VNC Server | `computer` | 空 | `AdwaitaLegacy/16x16/devices/computer.png`（同上） |
| Alacritty | `Alacritty` | 空 | **仍是空**（本机只有 `Alacritty.svg`；能力差，见 §5） |

`qemu` 类条目的 512 档同理（本机 `qemu.desktop` 不在扫描根里，用 pin 覆盖）。

## §3 台账

**V=0 / B=0 / S=2**。两条都是「结构真变了」：

- S①：`iconDirs()`（8 项写死目录）→ `iconBaseDirs()`（base）+ `themesIn(base)`（主题）+
  `findInTheme(themeDir, …)`（布局）。旧结构是「目录即主题」，新结构是「base 里枚举主题」。
- S②：`Icon=` 的 name 规则从「原样 + `.png`」变成 `iconName`（剥已知图标扩展名），
  且兜底走法多了一张每主题的 name→path 表。

没有视觉 pin 变红：本仓的图标没有像素 pin（`Icon` 的字节只经过 `DecodeBitmap`，测试不钉尺寸）。

## §4 门槛（HEAD 改后实测，`touch internal/apps/*.go` 后复跑）

```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
go test -count=1 ./...                                 → 全绿
--- PASS                                               → 2145（基线 2137，+8）
--- SKIP                                               → 15（=）
--- FAIL                                               → 0
测试函数（^func Test 计数）                              → 1747（基线 1739，+8）
包数                                                    → 36（=）
GOOS=linux/windows/darwin go build ./...                → 三条都过
```

本轮非视觉，不出图（无墨与面的改动；图标字节本身仍走原来的 `DecodeBitmap` 路径）。

## §5 变异（5 条实跑变红 + `md5sum -c` 逐字节还原）

原始报错串（`go test -count=1 -run <pin> ./internal/apps`，逐条注入后摘录；
每次 `cp` 还原后 `md5sum -c /tmp/p10s-md5.txt` 均「成功」）：

1. `findInTheme` 的 `theme/apps/<数字>` 布局换成 `size`（breeze 式布局消失）：
   `--- FAIL: TestTheThreeThemeLayouts` /
   `iconsearch_test.go:156: layout [apps 48] resolved to ".../hicolor/apps/p10s-l3.png", want ".../hicolor/apps/48/p10s-l3.png"`
2. `iconSizes` 的 `512x512` 与 `256x256` 换位：
   `--- FAIL: TestTheLargestSizeWins` /
   `iconsearch_test.go:180: iconSizes = [256x256 512x512 …], want the reference's [512x512 256x256 …]` +
   `iconsearch_test.go:186: resolved to ".../256x256/apps/p10s-big.png", want the 512x512 …`
3. `desktopIconFile` 的 `iconName(name)` 换回 `filepath.Base(name)`：
   `--- FAIL: TestTheIconNameLosesOnlyAKnownExtension` /
   `iconsearch_test.go:66: Icon="p10s-a.png" resolved to "", want "…/p10s-a.png"`（`.PNG`/`.svg`/`.xpm` 同串）
4. pixmaps 与主题两段搜索对调：
   `--- FAIL: TestPixmapsComeBeforeThemes` /
   `iconsearch_test.go:99: resolved to "…/hicolor/256x256/apps/p10s-both.png", want the pixmap "…/pixmaps/p10s-both.png"`
5. `searchIconDepth` 3 → 99：
   `--- FAIL: TestAnUnusualLayoutIsFoundThreeDeep` /
   `iconsearch_test.go:254: four-deep icon resolved to "…/a/b/c/d/p10s-toodeep.png", want nothing`

每条注入前 `md5sum internal/apps/icons.go` = `53f6b26be9d8883c768f8f3db8b92998`，还原后
`md5sum -c` 逐字节核对通过；最后 `touch internal/apps/*.go` 复跑 `./internal/apps
./internal/shell ./internal/launcher ./internal/inventory` 四条全绿（避开测试缓存）。

## §6 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 48 轮，本仓无可改代码。
2. 需拍板不变（`.pending-bugs.md` P9-g 表），外加 **#692 日语 locale**、**#642 `MYGO_LDFLAGS`**、
   `ShortcutTargetName` 对「无扩展名」返回 `""`（`shortcut.go:208-210`）。
3. register 只留 `cmd`、`open` 是否拒一切 query、未知宿主是否改成静默忽略，三条拍板项照旧。
4. 发布候选（命名先定）：`dist/SHA256SUMS`、macOS `install.sh`、Windows `install.ps1`。
5. **本轮新记的能力差（要拍板才做）**：SVG/XPM 图标。参考把 `ICON_EXTENSIONS = png, svg, xpm`
   copy 进缓存让前端渲染；本仓的渲染面是 `ui.DecodeBitmap`（只吃 PNG），所以
   `Alacritty`（本机只有 `/usr/share/pixmaps/Alacritty.svg`）继续显示首字母。
   要做就得引入 SVG 光栅化（`golang.org/x/image` 不含 SVG），属于渲染能力变更，
   写进拍板表，**本轮不擅自实现**。
6. 下一轮建议：图标这条线剩下的唯一**分歧**就是 §6.5 的 SVG（要拍板）。不需要拍板的相邻
   表面是 macOS 的一条回退缺口：`readIcon`（`icons.go:122-137`）在 `IconPath != ""` 时
   **只读那个文件**，`.icns` 解不出 PNG（老式 `is32`/`il32` RLE 条目，`LargestPNGFromICNS`
   返回 nil）或读失败就返回 nil，不再问平台；而 Windows 侧 `IconPath` 恒为空
   （`shortcut.go` 不设它），走的是 `extractIcon`。先量参考 `macos.rs:509-560` 在
   「icns 里没有 PNG 条目」时做什么，再决定改产品还是钉住。
