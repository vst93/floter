# P10-p 报告：升级 mygo v0.6.4 → v0.7.0（4 条破坏性变更逐条定性，唯一真命中修掉）

规格 `docs/rounds/P10-p-spec.md`。基线 HEAD `2d619e6`（P10-o），v0.6.4 实测
**PASS 2128 / SKIP 15 / FAIL 0 / 36 包 / 1730 测试函数**。

## ① 意图与证据

任务书「当前进度」把 mygo v0.7.0 标成 ⚠️ 必做（4 条破坏性变更），P10-n / P10-o 两轮走了别的
方向，这条欠了两轮。本轮只做「升级 + 破坏性变更适配」，不顺手重构。

### 1.1 Go API：只有新增

用两版模块目录对导出声明做 diff（根包 / `ui` / `audio` / `text` / `backend` / `platform`）：

- **删除的导出符号 0 个**。唯一一条「删除」是 `Application.activationPolicy()` →
  `ActivationPolicy()`（私有改公开）。
- 新增（本仓未用）：`NowPlayingModule` 全家、`MenuIcon`、`Page.IsAudible/SetAudioMuted`、
  `UpdaterModule.SetPrereleases`、`Window.SetActiveAppearance/SetWindowButtonsTheme`、
  `ui.Element.Cache/Hidden/DrawObjects`、`ui.Painter.AnimationFrameRate`、
  `ui.Path.Cap/Join/Dash/MiterLimit`、`ui.Context.SetZoom/Zoom`、`audio.SetLatency/OnEnd`。

实测：改完 `go.mod` 后 `go build ./...`、`go vet ./...` **零源码改动通过**，
`go test ./...` 一次全绿。

### 1.2 四条破坏性变更逐条定性

| # | 变更 | 本仓命中面（实测） | 结论 |
|---|---|---|---|
| 746 | `audio.Player.Position` 时序 | `audio.` 零命中；`shell.go` 的 `a.Win.Position()` 是窗口位置 | 不受影响 |
| 764 | 有 `Parent` 的子窗口不再 attach 成 AppKit child window | `grep -rn "Parent" --include=*.go .` 只命中 `internal/shell/linkmanifest.go:93` 的注释（`Component::ParentDir`）；三处 `mygo.NewWindow`（`pinned.go:42`、`detach.go:155`、`shell.go:1256`）都不设 `Parent`/`Modal`/`Flyout` | 零路径 |
| 768 | CJK 应用名默认 slug | `mygo.json` 无 `slug` 键、名字纯 ASCII → `slug()` == `floter`；deb 命名格式两版一字未改（`cmd/mygo/deb.go:203` 都是 `%s_%s_%s.deb`） | 产物名不变（实测见 1.3） |
| 769 | dmg / Windows installer 按 slug 命名 | dmg 由 `*.dmg` glob 收（`release-mygo.yml:387`）不受影响；**NSIS 名从 `floter Setup <v> amd64.exe` 变成 `floter-setup-<v>-amd64.exe`，而 `:427` 的 glob 还是 `*Setup*.exe`** | **断，已修** |

`#793 SearchField AutoFocus` 也顺手验了：全仓 `AutoFocus` 零命中，搜索框靠显式
`FocusSearch()`（`internal/launcher/launcher.go:1043`，8 个调用点），升级不改这条路径。

### 1.3 真实产物名（`go tool mygo build -platform linux/amd64`，升级后）

```
dist/linux-amd64/floter
dist/linux-amd64/floter_0.3.24_amd64.deb
dist/linux-amd64/floter-0.3.24-linux-amd64.tar.gz
dist/linux-amd64/install.sh
dist/SHA256SUMS                     ← v0.7.0 新增
```

与升级前同名：`build.go` 里传给 deb/tar/exe 的 `slugify(name)` 换成了 `c.slug()`，
对 `floter` 同值（`config.go:488-493`）。用户已装机的 `/opt/floter/floter`、
`~/.local/floter.app`、`/usr/bin/floter` 不变。

`windows/amd64` 交叉构建（本机无 makensis）拿到 `dist/windows-amd64/floter.exe`，
setup 名按 `nsis.go:99-108` 是 `floter-setup-0.3.24-amd64.exe`（v0.6.4 是
`floter Setup 0.3.24 amd64.exe`）。

## ② 落地

### 2.1 `go.mod` / `go.sum`

`github.com/egoist/mygo v0.6.4` → `v0.7.0`（`go get`，`go 1.27.3` 不变，间接依赖无变化）。

### 2.2 `release-mygo.yml`：setup 产物的 glob（唯一产品级修复）

`SETUP=$(ls "$D"/*Setup*.exe ...)` → `*-setup-*.exe`，并把「mygo ≥ v0.7.0 的命名规则」
写进注释。这不是风格问题：`[ -n "$SETUP" ] && cp ...` 在 glob 空的时候**不报错**，
升级后发布会少一个资产，而 Release Notes 表格和 `docs/release.md` 仍链着它 —— 用户点下载 404。

`windows-smoke.yml:44` 的 `-notmatch "Setup"` 不动：PowerShell 的 `-match` 不区分大小写，
`floter-setup-*.exe` 仍被滤掉。

### 2.3 新 pin `internal/ci/artifactnames_test.go`（+2 函数）

- `TestTheWindowsSetupGlobMatchesMygoNames`：从 workflow 里用正则抠出 `SETUP=$(ls "$D"/<glob>)`
  的 glob，`filepath.Match` 断言它匹配 `floter-setup-0.3.24-{amd64,arm64}.exe`；并断言 workflow
  里不再出现 `*Setup*`。**两侧一起钉**：改 glob 或改命名规则都会红。
- `TestTheAppNameKeepsArtifactNamesStable`：读 `mygo.json`，断言 `name` 纯 ASCII、slug 推导结果是
  `floter`。这是 #768 的护栏 —— 一旦名字带 CJK（现在会走拼音 slug）或加了 `slug` 覆盖，
  deb/dmg/tar/exe 的名字就全漂移，测试先红。

### 2.4 新特性里**没做**的（写清理由，不留 TODO 雾）

- `#753/#754/#757` 脏矩形 + `AnimationFrameRate` + `Element.Cache`：要逐元素标注，是性能轮。
- `#736 Glass.Clarity`、`#772 Element.Hidden`：前者改视觉（视觉轮只做用户实测反馈），后者是重构。
- v0.7.0 新增的 `dist/SHA256SUMS`、macOS `install.sh`、Windows `install.ps1` 未纳入发布：
  三个 matrix 各产一份 `SHA256SUMS`，合并发布时会同名冲突，命名要先拍板；`install.ps1`
  文档未提。**记进「仍待」，不擅自加。**

## ③ 台账

**V=0 / B=0 / S=0**。产品代码零改动；测试只**新增** 2 个函数，没有放宽/删除任何断言。
`go.mod`/`go.sum` 与 workflow 一行 glob 是本轮全部改动面。

## ④ 门槛（实测）

（见下方追加）
```
gofmt -l cmd internal                                  → 空
go vet ./...                                           → 净
go test -count=1 ./...                                 → 全绿（EXIT=0）
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 2130（基线 2128，+2）
grep -c -- "--- SKIP"                                  → 15（=）
grep -c -- "--- FAIL"                                  → 0
包                                                     → 36（34 ok + 2 no test files）
测试函数                                               → 1732（基线 1730，+2）
GOOS=linux|windows|darwin go build ./...               → 三平台全过
```

## ⑤ 变异（3 条，全部实跑变红 → 还原 → 逐字节核对 → `touch` 复跑）

**M1** `release-mygo.yml` 的 glob 改回 `*Setup*.exe`：

```
--- FAIL: TestTheWindowsSetupGlobMatchesMygoNames (0.00s)
    artifactnames_test.go:26: release-mygo.yml still globs `*Setup*`, which mygo >= v0.7.0 no longer produces
    artifactnames_test.go:47: the setup glob "*Setup*.exe" does not match the installer the build makes, "floter-setup-0.3.24-amd64.exe"
    artifactnames_test.go:47: the setup glob "*Setup*.exe" does not match the installer the build makes, "floter-setup-0.3.24-arm64.exe"
```

还原：`md5sum -c` → `成功`；`touch` 后复跑绿。

**M2** `mygo.json` 加 `"slug": "floter-dev"`：

```
--- FAIL: TestTheAppNameKeepsArtifactNamesStable (0.00s)
    artifactnames_test.go:95: mygo.json slugs to "floter-dev", but the release assets, install paths and docs are all named floter-*
```

还原：`md5sum -c` → `成功`；`touch` 后复跑绿。

**M3** `mygo.json` 的 `name` 改成 `floter 面板`（模拟 #768 的 CJK 路径）：

```
--- FAIL: TestTheAppNameKeepsArtifactNamesStable (0.00s)
    artifactnames_test.go:71: mygo.json name "floter 面板" is not ASCII: mygo >= v0.7.0 slugs it through pinyin, moving every artifact name
```

还原：`md5sum -c` → `成功`；`internal/ci` 复跑 `ok`。

## ⑥ 仍待

1. 用户实机复测四条（Windows 透明 / 三大金刚 / 卡死 / 几何日志），连续第 45 轮，本仓无可改代码。
2. 需拍板不变（`.pending-bugs.md` P9-g 表）+ `#692 日语 locale` + `#642 MYGO_LDFLAGS` +
   `ShortcutTargetName` 对无扩展名返回 `""` + register 只留 `cmd` + `open` 是否拒一切 query +
   未知宿主是否静默忽略。
3. 新拍板候选：v0.7.0 的 `dist/SHA256SUMS`（三 matrix 同名冲突）、macOS `install.sh`、
   Windows `install.ps1` 是否纳入发布 —— 命名要先定，本轮未动。
4. 下一轮建议：`internal/extensions/candidates.go` 的候选排名（P10-o 报告遗留的「唯一还没按参考
   逐条核对过的排序面」）。
