# P11-t 报告：用户键入/拖入的路径里，开头的 `~` 只有一个含义

规格 `docs/rounds/P11-t-spec.md`。方向按用户定的「三平台一致性 → 基础功能 → 像素细节」，
落在**三平台一致性**：`~` 的展开是 shell 的活，桌面应用得自己做；本仓做了四遍，给出两套
答案，其中一套在 Windows 上把 `~/notes` 变成 `C:\Users\<u>\~\notes`——而那一行正是动作栏
显示、`Copy` 复制、`Open` 打开的东西。

## 1. 做了什么

新增 `internal/homepath`（`homepath.go`，95 行）：

- `Expand(path, home) (string, bool)`：只处理开头的 `~`（`~`/`~/`/`~//x` → home；
  `~alice` → `home/~alice`；相对路径原样返回，留给调用点选基准；需要 home 而没有 → `false`）。
- `Resolve(path, home) (string, bool)`：`Expand` + 相对路径 `Join(home)`——参考
  `main:src-tauri/src/commands/actions.rs:92-105` 的 `resolve_path`。
- 分隔符判据用 `os.IsPathSeparator`（参考的 `std::path::is_separator`）：`~\x` 在 Windows 上
  是 `home\x`、在 POSIX 上是 `home/~\x`。这是参考自己的「每平台一个答案」，本轮不另发明。

四个调用点转发：

| 位置 | 旧 | 新 |
|---|---|---|
| `internal/shell/launch.go` `resolvePath`（打开路径 / 终端链接） | 33 行手写 | 一行转发 `homepath.Resolve` |
| `internal/launcher/actionbar.go` `expandPath`（动作栏路径行） | `FromSlash` 后 `CutPrefix("~/")` | 保留 `TrimSpace`+`FromSlash`（行标题用本机分隔符），展开转发 `Resolve` |
| `internal/drops/drops.go` `normalize`（拖入） | 15 行手写 | 一行转发 `homepath.Expand`（相对路径仍对 `cwd`） |
| `internal/shell/linkmanifest.go` `linkManifestValue`（连接链接的 manifest） | `Join(home, rest)` | 转发 `homepath.Resolve` |

## 2. 修掉的两条

1. **Windows 上动作栏的 `~/x` 是错路径**（用户可见）。旧代码先
   `filepath.FromSlash(value)`，Windows 上 `~/notes` 已成 `~\notes`，随后
   `strings.CutPrefix(expanded, "~/")` 落空，走「相对路径」分支 →
   `filepath.Join(home, "~\\notes")` = `C:\Users\<u>\~\notes`。行标题、`Copy`、`Open`
   全是这个路径。新读法先看 `~` 再看分隔符，`~/` 与 `~\` 在 Windows 上都认。
2. **连接链接的 `~alice` 是第二个答案**。`linkManifestValue` 把 `~` 之后的内容直接 join
   home，于是 `~alice/tool.json` → `home/alice/tool.json`；另外三处与参考都是
   `home/~alice/tool.json`（参考拍板：`~alice` 是「名字就叫 `~alice` 的目录」）。
   本轮统一到参考的答案。

## 3. V/B/S 台账

- **V = 0**：不碰墨与面。
- **B = 0**：既有断言零改动全绿。`internal/shell` 的 `TestResolvePathMatchesTheOldResolver`
  七行、`TestAWindowsSpelledPathIsStillHomeRelativeHere`、`TestOpenPathResolvesAndRequiresExistence`、
  `internal/drops` 的五条、`internal/launcher` 的动作栏/路径 memo 全部原样通过。
- **S = 2**：
  1. 结构：四处手写展开 → `internal/homepath`（旧结构 → 新结构）。
  2. `linkManifestValue` 的 `~alice` 答案：`home/alice` → `home/~alice`
     （`linkmanifest_test.go` 新增一行，注释 `P11-t 重定基线：旧值 home/alice → home/~alice`；
     旧代码没有这一行的 pin）。

新增测试函数 6 条：`internal/homepath/homepath_test.go` 3 条、
`internal/launcher/pathaction_test.go` 2 条、`internal/ci/homepath_contract_test.go` 1 条；
另在 `linkmanifest_test.go` 的表里加一行（不是新函数）。

## 4. 门槛实测

- `gofmt -l cmd internal`：**空**
- `go vet ./...`：净 · `GOOS=windows go vet ./...`：净 · `GOOS=darwin go vet ./...`：净
- `go test -count=1 ./...`：**0 FAIL**
- PASS：**2315**（起点实测 2309）
- 测试函数：**1874**（起点实测 1868）
- SKIP：**15**（不变）· FAIL：**0**
- 三平台 `go build ./...`：过
- 行数：**42**（40 ok + 2 no test files）；本轮新增 1 个包，起点是 41
  （任务书里的 38 与起点实测不符，以 `go list ./...` 为准）

非视觉轮，不出图。

## 5. mutation（2 条实跑，先红后逐字节还原）

注入文件 `internal/homepath/homepath.go`（`md5 f3df18be…`），两次 `md5sum -c` 均「成功」，
还原后 `touch` 再复跑，`./internal/homepath ./internal/launcher ./internal/shell ./internal/drops ./internal/ci` 全绿。

1. `Expand` 的 `~alice` 分支改回 linkmanifest 的旧答案（`Join(home, rest)`）：
   - `homepath_test.go:43: Expand("~alice") = ".../001/alice", want ".../001/~alice": a directory literally called ~alice, not another user`
   - `pathaction_test.go:45: expandPath("~alice") = ".../001/alice", want ".../001/~alice"`
   - `launch_test.go:294: resolvePath("~alice") = ".../alice", want ".../~alice"`
   - `linkmanifest_test.go:71: "~alice/tool.json" resolved to "/home/tar/alice/tool.json", want "/home/tar/~alice/tool.json"`
2. `Resolve` 删掉「相对路径 Join(home)」：
   - `homepath_test.go:85: Resolve("notes/a.md") = "notes/a.md", want ".../001/notes/a.md"`
   - `pathaction_test.go:45: expandPath("notes") = "notes", want ".../001/notes"`
   - `launch_test.go:294: resolvePath("notes/a.md") = "notes/a.md", want ".../001/notes/a.md"`
   - `linkmanifest_test.go:71: "tool.json" resolved to "tool.json", want "/home/tar/tool.json"`
   - `linkmanifest_test.go:96: resolved to "C:\\tools\\tool.json", want "/home/tar/C:\\tools\\tool.json"`
   - `launch_test.go:255: opened = [.../note.txt], want [.../note.txt .../note.txt]`

规格里的第 3 条（`os.IsPathSeparator` → `"/"`）没有实跑：它在 Linux 门禁上看不到，
只有 Windows CI 那一格会红，本轮按 2 条交付。

## 6. 仍待（P11-t）

1. 用户实机复测四条，连续第 75 轮。
2. 需拍板清单不变（P9-g 表 + 剪贴板轮询 600 vs 900 + 复制回来是否置顶 + 计算器
   `loadLocked` + `usage.Record` 写失败是否回滚 + 「搜索页答案行」+ 非 macOS 快捷键词序
   + `Esc`/`Escape` + SVG/XPM 图标 + icns 落盘缓存 + 发布 `dist/SHA256SUMS`
   + P11-s 的 `Name` 缺失回落文件名主干 + `Hidden`/`NoDisplay` 大小写）。
3. `~` 展开已收口；`extensions/sync.go` 的 `HasPrefix(text, "~")` 是分类（设备路径）、
   `launch.go` 终端链接的 `HasPrefix(path, "~")` 是「像不像绝对路径」，两者都不是展开，
   契约扫描不覆盖它们（注释里写明）。
4. 下一处候选：`internal/inputtext` 已被侦察结论标为「与参考同答案，不是缺陷」；
   建议往 `internal/i18n` 的 locale 匹配（`desktopentry` 的语言阶梯 vs i18n 的
   locale 解析是否同一套）量 pin 后再动。
