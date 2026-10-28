# P11-t 规格：用户键入/拖入的路径里，开头的 `~` 只有一个含义

方向按用户定的「三平台一致性 → 基础功能 → 像素细节」，落在**三平台一致性 + 基础功能**：
`~` 是 shell 的写法，桌面应用要自己展开；这件事本仓现在有**四个实现、两套答案**，
其中一套在 Windows 上把 `~/notes` 展开成 `C:\Users\<u>\~\notes`。

## 1. 意图

「用户写 `~/x` 指的是哪个文件」只该有一个答案。参考实现把它定在一个函数里
（`main:src-tauri/src/commands/actions.rs:92-105` 的 `resolve_path`，动作栏与打开路径
都走它），本仓却有四份手写实现：

| 位置 | 现状 | 问题 |
|---|---|---|
| `internal/shell/launch.go:78-100` `resolvePath` | `~`/`~/`/`~alice`/相对路径全部按参考处理 | 正确的一份 |
| `internal/drops/drops.go:53-67` `normalize` | 同样规则（注释里写明「exactly the rule `resolve` in the reference follows」） | 正确，但是第二份 |
| `internal/launcher/actionbar.go:297-311` `expandPath` | 先 `filepath.FromSlash`，再 `strings.CutPrefix(expanded, "~/")` | **Windows 上 `~/x` 认不出来**：`FromSlash` 已把 `~/x` 变成 `~\x`，`CutPrefix("~/")` 落空，走相对分支 → `Join(home, "~\\x")` = `C:\Users\<u>\~\x`。动作栏那一行的标题、`Copy` 复制的、`Open` 打开的都是这个错路径 |
| `internal/shell/linkmanifest.go:75-87` `linkManifestValue` | `filepath.Join(home, rest)` | `~alice` → `home/alice`，另外三处是 `home/~alice`（参考的拍板：`~alice` 是「名字就叫 `~alice` 的目录」） |

`expandPath` 的 Windows 错误是**用户可见**的：动作栏对 `~/Documents` 显示
`C:\Users\u\~\Documents`，复制走同一串。第三处分歧（`~alice`）只在连接链接的 manifest
值上可达。

## 2. 落地项

新增 `internal/homepath`（一个文件 + 一个测试文件），导出两个函数：

- `Expand(path, home string) (string, bool)`：只处理开头的 `~`。
  - 不是 `~` 开头 → 原样返回，`true`。
  - 是 `~` 且 `home == ""` → `false`（不能把 `~` 静默变成相对路径）。
  - `~alice`（`~` 后面不是分隔符）→ `filepath.Join(home, path)`，即 `home/~alice`。
  - `~`、`~/`、`~//x` → 先剥掉所有分隔符，空则 `home`，否则 `Join(home, rest)`。
  - 分隔符判据用 `os.IsPathSeparator`（参考用 `std::path::is_separator`）：
    `~\x` 在 Windows 上是 `home\x`，在 POSIX 上是 `home/~\x`——与参考同一处分歧。
- `Resolve(path, home string) (string, bool)`：`Expand` 之后，非绝对路径 `Join(home, …)`；
  需要 home 而 home 为空 → `false`。这就是参考的 `resolve_path`。

四个调用点转发：

1. `internal/shell/launch.go` `resolvePath` → `homepath.Resolve`（`ok=false` 时返回原串，
   `openPath` 的 `os.Stat` 会拒掉它并给出提示，行为不变）。
2. `internal/launcher/actionbar.go` `expandPath` → 保留 `TrimSpace` + `filepath.FromSlash`
   （Windows 上让行标题用本机分隔符），随后 `homepath.Resolve`；`ok=false` 时返回
   `FromSlash` 后的串。**修掉 `~/x` 在 Windows 上变成 `home\~\x`。**
3. `internal/drops/drops.go` `normalize` → 用 `homepath.Expand`（相对路径仍对 `cwd` 解析，
   这是拖入的语义，与动作栏不同）。
4. `internal/shell/linkmanifest.go` `linkManifestValue` → 用 `homepath.Resolve`；
   `~alice/x` 由 `home/alice/x` 改成 `home/~alice/x`（与另外三处、与参考同答案）。
   绝对路径仍然不需要 home（`os.UserHomeDir` 失败不影响它们）。

## 3. 行为契约零改动红线

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**不动**。
- 动作栏的 `~` 展开结果（Linux 上）逐字节不变：`~`→home、`~/x`→home/x、
  `~//x`→home/x、`~alice`→home/~alice、相对→home/相对、绝对不变。
- 拖入的路径：`~alice` 仍是 `home/~alice`，相对仍对 `cwd`，`Clean` 与
  `symlink_metadata` 判据不变；「不 trim 再解析」的拍板（`drops_test.go:87`）不变。
- 终端链接的 `~` 判据（`launch.go:271` 的 `HasPrefix(path, "~")`）只是「像不像绝对路径」，
  不是展开，保留原样。
- `extensions/sync.go:230` 的 `HasPrefix(text, "~")` 是分类（设备路径），不是展开，保留。

## 4. 门槛命令与 PASS 基线（本轮起点 HEAD `6a20f01` 实测）

```
gofmt -l cmd internal                                  # 期望空
go vet ./...                                           # 期望净
GOOS=windows go vet ./...                              # 期望净
GOOS=darwin  go vet ./...                              # 期望净
go test -count=1 ./...                                 # 期望 0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 基线 2309
grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l   # 基线 1868
GOOS=linux/windows/darwin go build ./...               # 期望过
```

本轮新增：`internal/homepath/homepath_test.go`（1 条表驱动）、
`internal/launcher/pathaction_test.go` 里补一条（动作栏行的标题是展开后的路径）、
`internal/shell/linkmanifest_test.go` 补一条 `~alice` 用例、
`internal/ci/homepath_contract_test.go`（1 条源码扫描）。

## 5. mutation 计划（≥2，每条说明会让哪个 pin 变红）

1. `homepath.Expand` 的 `~alice` 分支改成 `filepath.Join(home, rest)`（即 linkmanifest 的旧答案）
   → `internal/homepath` 的表驱动 `~alice` 行、`internal/shell` 的
   `TestResolvePathMatchesTheOldResolver`、`internal/shell/linkmanifest_test.go` 的新用例变红。
2. `homepath.Resolve` 把「相对路径 Join(home)」删掉（原样返回相对路径）→
   `TestResolvePathMatchesTheOldResolver` 的相对行、`internal/launcher` 的新 pin 变红。
3. （若时间允许）`Expand` 用 `"/"` 代替 `os.IsPathSeparator` 判分隔符 → Windows 上
   `~\x` 那行变红（Linux 门禁看不到，靠 Windows CI）。
