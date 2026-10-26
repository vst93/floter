# P10-j 规格：`connect` 的 manifest 值也有判据——本仓收的形状写成一个函数

## 意图（一句话）

`floter://connect?manifest=…` 的值今天只有一条判据（非空，`internal/shell/system.go:211-215`），
其余全交给 `os.Stat` 和 `extensions.PrepareLocal` 去碰运气；参考对同一个值有六条结构判据
（`main:src-tauri/src/deep_link.rs:288-338`）。本轮把这六条逐条定性：**能接的接**（空、控制字符、
`..` 上溯、显式 scheme），**接了会删掉本仓已文档化行为的钉住**（`~`、home 相对、目录形态、
`https` 远程 manifest 本仓没有落地路径），并把「值 → 要 stat 的路径」收成一个纯函数，让
`HandleURL` 与 `connectFromLink` 不再各判一半。

## 为什么现在做（证据）

- 交棒条目就是这条（`.pending-bugs.md` P10-i「仍待」/ 任务书「现在该做什么」）。
- 现状（本轮实测行号）：
  - `internal/shell/system.go:211-215`：只 `TrimSpace` + 非空 → 直接 `a.Open(SurfaceSettings)` +
    `connectFromLink`。空值是唯一在「碰设置页之前」被拒的形状。
  - `internal/shell/shell.go:2374-2382`：`~` 展开、相对路径按 home 拼、`filepath.IsAbs` 三件套内联在
    后台 goroutine 里；`os.Stat` 失败才拒（`:2383-2388`）；**文件形态**只要 stat 得到就取它的目录
    （`:2389-2394`），值里的文件名从不参与判断。
  - 于是今天 `floter://connect?manifest=/tmp/notes.txt`（`/tmp` 里恰好有 `floter.extension.json`）
    会照常安装 `/tmp`；`manifest=/home/u/../../etc/x.json` 会照常 stat 上溯后的路径；
    `manifest=file:///home/u/x.json` 会被当成 home 下的一个怪文件名去 stat。参考对这三条都是
    结构拒绝（`ManifestNotJson` / `ManifestTraversal` / `ManifestInsecure`），且都不在
    `is_silent` 名单里（`deep_link.rs:225-228`）→ 用户会看到一句话。
- 已文档化、**不能删**的行为（这是本轮的分界线）：
  - `docs/mygo-rewrite-plan.md:690-695`：「manifest 路径（支持 `~`，相对路径按 home 解析）…
    manifest 可以是文件（包取其所在目录）或目录本身」。
  - `internal/shell/shell_test.go:3321-3326` 的 pin 与注释（「may name a directory or its manifest
    file」）、`internal/shell/connectlink_test.go:68/115/177` 三条测试都拿**目录**当 manifest。
  - `internal/shell/linkargs.go:78-82`：终端拼写 `floter connect ./tool.json` 折成同一条链接，
    所以「相对路径」这条也在 CLI 上活着。
  - 参考的 `https` 远程 manifest 分支在本仓**没有落地路径**：`extensions.PrepareLocal`
    （`internal/extensions/install.go:68`）是 connect 的唯一入口，只吃目录；npm 那条走
    `PrepareRegistry`（`install.go:83`，按名字+版本约束装，不吃 manifest URL）。

## 落地项（位置 → 现状 → 应改成）

### 1. 新 `internal/shell/linkmanifest.go`

- 现状：不存在；判据散在 `system.go:211-215`（空）与 `shell.go:2374-2394`（展开 + stat + 目录推断）。
- 改成两个函数：

  `linkManifestValue(raw string) (string, bool)` —— 值 → 要 stat 的路径，纯函数，逐条：

  1. `TrimSpace`；空 → 拒（参考 `MissingManifest`，`deep_link.rs:290-292`）。这条接管
     `system.go:211-215` 的判据，`HandleURL` 不再自己判空。
  2. 含控制字符 → 拒（`ManifestInsecure`，`:296-299`；参考的理由是 NUL 会在到达文件系统或
     header 的那一刻截断值）。今天这类值会一路走到 `os.Stat`。
  3. 含 `://` → 拒（参考只接受 `https` 远程 manifest，`:301-317`）。**分歧钉住**：本仓没有远程
     manifest 的落地路径（`PrepareLocal` 是唯一入口），所以 `https://…` 也在**碰设置页之前**拒，
     而不是像今天这样落到 stat 失败。参考对 `file://` / `http://` / 带凭据的 `https://` 同样拒。
  4. `~` / `~/…` → home（`:690-695` 的承诺，原样搬过来）。
  5. 相对路径 → 按 home 拼（同上）。
  6. 任一 `..` 组件 → 拒（参考 `ManifestTraversal`，`:319-333`）。**判据比参考更统一**：参考用
     `Component::ParentDir`，在 POSIX 上 `\` 不是分隔符、在 Windows 上是，于是同一个值在三个平台
     可能两个答案；本仓按 `/` 与 `\` **两种分隔符都切**，三平台一个答案（与 P9-w/P9-x/P9-z 的
     「三平台只留一个答案」同一条纪律）。`..notes.json` 这种**文件名**不算上溯（参考自己也有这条
     pin，`deep_link.rs:1597-1600`）。

  `manifestFileFits(path string) bool` —— 文件形态的 `.json` 判据（参考 `ManifestNotJson`，
  `:334-336`）。**只对文件形态**生效：目录形态是本仓文档化的包形态（`PrepareLocal` 的入参就是
  目录），给它加 `.json` 要求会删掉文档化行为；参考那边没有目录形态，所以这条不是分歧而是
  「本仓多出来的形态豁免」。放在 stat **之后**（要 `info.IsDir()`），与今天
  `shell.go:2389-2394` 的目录推断同一处。

- 注释里写清三件事：参考的六条判据逐条落到哪、哪两条是**故意分歧**（`~`/相对/目录 vs 参考的
  绝对路径要求；`https` 远程 vs 本仓无落地路径）、以及为什么 `.json` 只管文件形态。

### 2. `internal/shell/system.go` 的 connect 分支（`:205-218`）

- 现状：`manifest := strings.TrimSpace(...)`；`manifest == ""` → `refuseLink`；否则开页 + 连接。
- 改成：`manifest, ok := linkManifestValue(parsed.Query().Get("manifest"))`；`!ok` → `refuseLink`
  并返回（**不碰设置页**，与 P10-h/P10-i 的闸门同一位置纪律）；`ok` 才 `a.Open` + `connectFromLink`。
  拒绝复用 `refuseLink` → `settings.deepLinkRejected`，**零新文案**。

### 3. `internal/shell/shell.go` 的 `connectFromLink`（`:2364-2394`）

- 现状：自己展开 `~`/相对路径；`!info.IsDir()` 就取 `filepath.Dir`。
- 改成：值已经是 `linkManifestValue` 解出来的路径（注释写明「唯一答案在 linkmanifest.go」），删掉
  内联的展开三分支；文件形态补 `manifestFileFits(expanded)`，不满足 → `refuseLink` 并返回。
  目录形态不变（继续当包目录）。函数注释同步改写：不再承诺「相对路径按 home 解析」（那是
  `linkManifestValue` 的承诺），改为指向它。

### 4. `docs/extensions/README.zh-CN.md` 补一节「从链接接入扩展包（`floter://connect`）」

- 现状：README 只有 `register` 一节（`:97-106`），connect 的形状只在 `docs/mygo-rewrite-plan.md`
  （内部计划）里。用户拿到的唯一线索是设置页里的示例串（`settingsui.go:3926`）。
- 改成：在 `register` 一节**之前**补一节，写清：绝对路径或 `~`/home 相对路径；不许 `..` 上溯；
  可以是包目录或它的 manifest 文件（文件形态须 `.json`）；不接受 URL（本版没有远程 manifest 的
  安装路径）；不满足的链接整条拒绝、说一句话、什么都不装。**承诺不变**：接入仍要用户批准权限。

## 行为契约零改动红线

- 高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**零改动**（无新文案、无新视图、
  无新接线；拒绝复用既有 `settings.deepLinkRejected`）。
- `register` 分支与 `linkparams.go` 的表：**零改动**（本轮只碰 `connect` 的值）。
- 终端拼写 `floter connect <path>` 的折叠（`linkargs.go:78-82`）：**零改动**——它只是把值塞进
  `manifest=`，判据在路由层，两处不重复。
- `extensions.PrepareLocal` 与安装/审查/通知链：**零改动**（本轮只决定「哪个目录送进去」）。

## 门槛命令与 PASS 基线（本轮实测）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 2099（本轮基线）
GOOS=linux/windows/darwin go build ./...               # 三个都过
```

基线（本轮实跑，不是旧报告数字）：**2099 PASS** / **36 包** / **15 SKIP** / **0 FAIL** /
**1709 测试函数**。

## 台账预判

- **V=0**（无视觉数值）。
- **B=0**（文案、接线、审查链零改动；现有 pin 全部照旧通过——三条 connect 测试都用绝对目录/文件）。
- **S=0**（没有结构断言要改）。

## mutation 计划（≥2 条，每条说清哪个 pin 变红）

1. 把 `..` 组件判据删掉 → `manifest=/home/u/../../etc/tool.json` 与 `~/a/../b.json` 两格变红。
2. 把控制字符判据删掉 → `manifest=/home/u/to\nol.json` 那格变红。
3. 把 `://` 判据删掉 → `https://example.com/tool.json` 那格变红（会退化成 home 拼接）。
4. 把 `manifestFileFits` 删掉 → 「`/pkg/notes.txt` 被拒」那条活路径测试变红。
5. 把 `linkManifestValue` 的 `~` 展开删掉 → `~/x.json` 解析结果那格变红。
6. 把 `system.go` 的调用点换回 `strings.TrimSpace` → 活路径测试
   `TestTheConnectLinkRefusesAManifestThatIsNotAPath` 变红（证明判据真的接在活路径上）。
