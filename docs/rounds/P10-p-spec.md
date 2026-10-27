# P10-p 规格：升级 mygo v0.6.4 → v0.7.0（4 条破坏性变更逐条定性 + 唯一真命中修掉）

## 意图

把 `mygo` 从 v0.6.4 升到 v0.7.0，并把 v0.7.0 四条破坏性变更里**唯一真正命中本仓**的一条修掉：
`#769` 把 NSIS 安装包改名成 `<slug>-setup-<version>-<arch>.exe`（小写 `setup`），而
`.github/workflows/release-mygo.yml:427` 的 glob 还是 `*Setup*.exe` —— 升级后**发布流水线会静默
漏掉 Windows 安装包**（`[ -n "$SETUP" ] && cp ...` 不报错，只是没有那个资产，Release Notes 表格
里的 `floter-<v>-windows-x86_64-setup.exe` 直接 404）。

为什么现在做：任务书「当前进度」把 v0.7.0 标成 ⚠️ 必做项；P10-n / P10-o 两轮走了别的方向，
这条已经欠了两轮。升级本身零源码改动（见下），风险全在产物名。

## 证据（动手前实测，行号是当前工作树）

### Go API：只有新增，没有删除/改签名

`diff` 两版模块的导出声明（根包 / `ui` / `audio` / `text` / `backend` / `platform`）：

- 删除的导出符号：**0 个**（唯一一条 `<` 是 `Application.activationPolicy()` → `ActivationPolicy()`，
  私有改公开，非破坏）。
- 新增：`Application.IsActive`、`NowPlayingModule` 全家、`MenuIcon`、`Page.IsAudible/SetAudioMuted`、
  `UpdaterModule.SetPrereleases`、`Window.SetActiveAppearance/SetWindowButtonsTheme`、
  `ui.Element.Cache/Hidden/DrawObjects`、`ui.Painter.AnimationFrameRate`、`ui.Path.Cap/Join/Dash/MiterLimit`、
  `ui.Context.SetZoom/Zoom`、`audio.SetLatency`、`audio.Player.OnEnd` 等。

实测：改完 `go.mod` 后 `go build ./...` 与 `go vet ./...` **零改动通过**。

### `#746 audio.Player.Position` → 不受影响

全仓 `audio.` 零命中；`internal/shell/shell.go` 里的 `a.Win.Position()` 是窗口位置。

### `#764` 子窗口随父窗口移动（macOS）→ 不受影响

`grep -rn "Parent" --include=*.go .` 只有一条注释命中（`internal/shell/linkmanifest.go:93`
的 `Component::ParentDir`）。本仓的 pinned 窗与 detach 窗都是**普通顶层窗口**，不设 `Parent`：
`internal/shell/pinned.go:42`、`internal/shell/detach.go:155`、`internal/shell/shell.go:1256`
三处 `mygo.NewWindow` 都没有 `Parent`/`Modal`/`Flyout`。v0.7.0 的变更是「有 `Parent` 的窗口不再
attach 成 AppKit child window」，对本仓零路径。

### `#768` CJK 应用名默认 slug → 产物名不变（实测）

v0.7.0 把 slug 当产物名的唯一来源：`cmd/mygo/config.go:488-493` `slug()` = `Config.Slug`，
否则 `slugify(executableName())`。`mygo.json` 没有 `slug` 键，应用名 `floter` 纯 ASCII
→ slug == `floter`。v0.6.4 用的是 `slugify(name)`（`build.go:323,327,366`），同一结果。

实测 `go tool mygo build -platform linux/amd64` 产物（升级后）：

```
dist/linux-amd64/floter
dist/linux-amd64/floter_0.3.24_amd64.deb
dist/linux-amd64/floter-0.3.24-linux-amd64.tar.gz
dist/linux-amd64/install.sh
dist/SHA256SUMS            ← v0.7.0 新增
```

deb 命名格式两版一字未改（`cmd/mygo/deb.go:203` 都是 `%s_%s_%s.deb`），传入的 `name`
从 `slugify(name)` 变 `c.slug()`，对 `floter` 同值。**用户已装机的路径（`/opt/floter/floter`、
`~/.local/floter.app`、`/usr/bin/floter`）不变。**

### `#769` dmg / Windows installer 按 slug 命名 → 命中一处

| 产物 | v0.6.4 | v0.7.0 | 本仓引用 | 结论 |
|---|---|---|---|---|
| dmg | `floter 0.3.24 universal.dmg`（`dmg.go:22-29`） | `floter-0.3.24-universal.dmg`（`dmg.go:22-31`） | `release-mygo.yml:387` `*.dmg` glob | 不受影响 |
| NSIS setup | `floter Setup 0.3.24 amd64.exe`（`nsis.go:98-101`） | `floter-setup-0.3.24-amd64.exe`（`nsis.go:99-108`） | `release-mygo.yml:427` `*Setup*.exe` | **断**：glob 大小写不再匹配 |
| tar.gz | `<name>-<v>-<target>.tar.gz` | `<slug>-<v>-<target>.tar.gz` | `release-mygo.yml:391,433` `floter-*-...` | 同值 |
| deb | `%s_%s_%s.deb` | 同 | `release-mygo.yml` `*.deb` | 同值 |

`windows-smoke.yml:44` 的 `-notmatch "Setup"` 是 PowerShell **不区分大小写**的 `-match`，
仍能滤掉 `floter-setup-*.exe`，不改。

### `#793 SearchField AutoFocus` → 不受影响

全仓 `AutoFocus` 零命中（唯一一条是 `internal/settingsui/uninstall_test.go:240` 的注释）。
搜索框靠显式聚焦：`internal/launcher/launcher.go:1043` `FocusSearch()`，调用点
`launcher/config.go:72`、`power.go:85`、`launcher.go:1055,1380,1951`、`shell.go:1395,3443`。
升级不改这条路径。

### 新特性里**不采纳**的（本轮只升级，不顺手重构）

- `#753/#754/#757` 脏矩形 + `AnimationFrameRate` + `Element.Cache`：需要逐元素标注，属性能轮。
- `#736 Glass.Clarity`、`#772 Element.Hidden`：改视觉/改结构，视觉轮只做用户实测反馈。
- v0.7.0 新增的 `dist/SHA256SUMS`、macOS `install.sh`、Windows `install.ps1`：三个 matrix 各自产一份
  `SHA256SUMS` 会在合并发布时同名冲突，命名要先拍板；`install.ps1` 文档未提。记进报告「仍待」。

## 落地项（位置 → 现状 → 应改成）

1. `go.mod:5` / `go.sum`：`mygo v0.6.4` → `v0.7.0`（已 `go get`）。
2. `.github/workflows/release-mygo.yml:427`：
   现状 `SETUP=$(ls "$D"/*Setup*.exe 2>/dev/null | head -1 || true)`
   → 改成 `*-setup-*.exe`，注释写明「mygo ≥ v0.7.0 用 `<slug>-setup-<version>-<arch>.exe`，
   旧的 `<Name> Setup <version> <arch>.exe` 已不再产出」。
3. 新 pin `internal/ci/artifactnames_test.go`（2 个测试函数）：
   - `TestTheWindowsSetupGlobMatchesMygoNames`：从 workflow 里抠出 `SETUP=$(ls "$D"/<glob>)` 的
     glob，用 `filepath.Match` 断言它匹配 v0.7.0 的真实产物名 `floter-setup-0.3.24-amd64.exe`，
     并断言 workflow 里不再出现 `*Setup*`（旧拼写）。
   - `TestTheAppNameKeepsArtifactNamesStable`：读 `mygo.json`，断言 `name` 是纯 ASCII 且没有
     `slug` 覆盖（`#768` 的 CJK 默认 slug 变更因此动不了本仓产物名）。
4. `docs/rounds/P10-p-report.md`：增量写，含实测数字与变异台账。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**一行不动**。本轮只碰 `go.mod`/
`go.sum`、一个 CI 工作流的一行 glob、`internal/ci` 的两个新测试函数。产品代码零改动。

## 门槛命令 + PASS 基线（HEAD `2d619e6`，v0.6.4 实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 2128（基线，只许涨）
```

基线实测：**PASS 2128 / SKIP 15 / FAIL 0 / 36 包（34 ok + 2 no test files）/ 1730 测试函数**。
升级后目标：PASS ≥ 2130（+2 新 pin），函数 1732，SKIP 15 不变，FAIL 0。

三平台：`GOOS=linux|windows|darwin go build ./...` 全过。

## mutation 计划（每条说明会让哪个 pin 变红）

1. 把 `release-mygo.yml:427` 的 glob 改回 `*Setup*.exe`
   → `TestTheWindowsSetupGlobMatchesMygoNames` 红（`filepath.Match("*Setup*.exe", "floter-setup-0.3.24-amd64.exe")` 为 false）。
2. 往 `mygo.json` 加 `"slug": "floter-dev"`
   → `TestTheAppNameKeepsArtifactNamesStable` 红（slug 覆盖会改 deb/tar/exe 名）。
3. 把 `mygo.json` 的 `name` 改成含非 ASCII 的名字（如 `"floter 面板"`）
   → 同一个 pin 红（`#768` 之后 CJK 名会走拼音 slug，产物名漂移）。

每条都要：注入 → 看到红 → 还原 → `md5sum -c` 逐字节核对 → `touch` 后复跑。
