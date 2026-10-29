# P11-bh 报告 · 文件页的空态说自己那件事

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完；`Super+N` 编号已由 P11-bb 收完）→
合约未落地条目（§6 全表 `已落地`，只剩 `⌘K`＝拍板）→ 上一轮「仍待」（P11-bg 第 1 条全是
老拍板项，第 2 条覆盖带、第 3 条侦察结论）→ **界面质感与交互逻辑**（用户 2026-10-28 定的
优先档）。规格 `docs/rounds/P11-bh-spec.md`。

## 1. 根因

`emptyMessage`（`internal/launcher/launcher.go`）是一个 switch：插件模式的每一种
「为什么空」都有自己的句子，`case a.files` 是唯一借邻居的那一个——

```go
case a.files:
    return copy.NoResults
```

于是「进到 `files` 模式、还没有拖入任何文件」这一页写着 **"No results" / "没有匹配项"**。
这句话的意思是「你查的东西不存在」，而这一页出现时**什么都没查**：用户是打了 `files`
（或 `file` / `drop` / `drops` / `文件`）进来的，手上一个文件都没拖。

同族的另外三张页都各有自己的一句（`ClipboardEmpty`、`CalculatorEmpty`、
`BrowserEmpty`/`BrowserDisabled`/`BrowserNoProfile`），所以这是**一张页两件事共用一个
答案**，而不是「文案要不要改」的口味问题。

## 2. 落地

1. `i18n.Launcher.FilesEmpty` 新增：en `"No files dropped yet"`、zh `"还没有拖入文件"`
   （`internal/i18n/i18n.go` 的字段注释 + `internal/i18n/copy.go` 两份文案）。
2. `emptyMessage` 的 `case a.files`：
   - `len(a.Dropped) == 0` → `copy.FilesEmpty`；
   - 有拖入文件、针没匹配上 → 仍是 `copy.NoResults`（那**是**失败查询）。
3. **渲染零改动**：墨（`tk.TextStrong`）、字阶（`tk.Emphasis`）、居中、`listEdge` 的
   位置全部不动。这也意味着高度链、键盘模型、命令接线零改动。

## 3. 台账（V/B/S）

- **V = 0**：没有钉过旧数值的 pin（`filters_test.go` 的 `emptyMessage` 表只量 calculator /
  clipboard / browser 与普通页，文件页从没有断言）。
- **B = 0 / S = 0**：既有测试**零改动**全绿（`go test -count=1 ./...` FAIL 0）。
- 新增 2 个测试函数：`TestTheFilesPageSaysWhyItIsEmpty`（两臂 × 两语言）与 opt-in 的
  `TestDumpP11BHFilesPNGs`。函数 1987 → **1989**，PASS 2442 → **2443**，FAIL 0，
  SKIP 43 → **44**（新增的 opt-in dump）。

## 4. 变异（3 条实跑变红 + `md5sum -c` 还原 + `touch` 复跑绿）

| # | 注入 | 原始报错串 |
|---|------|-----------|
| M1 | `if len(a.Dropped) == 0 { return copy.FilesEmpty }` 整段退回 `return copy.NoResults` | `filesempty_test.go:43: en: the empty files page does not say "No files dropped yet"; it drew ["No results" "Search" "Type to search" "Settings"]` |
| M2 | en 的 `FilesEmpty` 改成 `"No results"` | 同上（第一臂：既没有那句自己的话，也仍然借了 `NoResults`） |
| M3 | `if len(a.Dropped) == 0` 恒真 | 规格里写了，本轮**没有实跑**（M1/M2 已证明第一臂灵敏；M3 只看第二臂，留作下轮若动这段时的检查项） |

M1/M2 都用 `cp` 还原后 `md5sum -c /tmp/p11bh.md5` 逐字节核对（两份源码 `成功`），
`touch` 两个源文件 + 测试文件再跑一次绿（`ok floter/internal/launcher`）。

## 5. 视觉证据（我自己复核了像素，不是只看测试数字）

`/tmp/floter-P11-bh-files-{light,dark}.png`（`FLOTER_PNG=1 go test -run
TestDumpP11BHFilesPNGs ./internal/launcher/`，720×488）。

**我在图上量到的**：

- 句子矩形 en `{X:307 Y:259 W:106 H:18}`、zh `{X:318 Y:259 W:84 H:18}`——正好居中在
  **列表会占的那条带**里（字段带 0–56、chips 60 起、面板高 488），不是贴在卡片顶边；
- 该矩形内落在底色之外的字墨像素：en 浅 **1908** / 深 **1902**，zh 浅 **1504** / 深 **1507**
  ——两种外观都真的画出来了（不是「文本树里有、屏上没有」）；
- 修前同一页画的是 `"No results"`（M1 的报错串把修前的整串文本抄了出来：
  `["No results" "Search" "Type to search" "Settings"]`），修后是
  `"No files dropped yet"` / `"还没有拖入文件"`；除这一句外，页面上其余的字段行、magnifier、
  页脚一个像素没动（渲染代码零改动）。

## 6. 门槛（实测）

```
gofmt -l cmd internal            # 空
go vet ./...                     # 净
GOOS=windows go vet ./...        # 净
GOOS=darwin  go vet ./...        # 净
go test -count=1 ./...           # FAIL 0
PASS 2443（基线 2442）| SKIP 44（43 + 1 条 opt-in dump）| 测试函数 1989（1987 + 2）
GOOS=linux/windows/darwin go build ./...   # 过
```

## 7. 侦察记录（本轮量过，别当 TODO 重领）

1. **滚轮在搜索框那条带上不滚列表**（真 bug，本轮没修）：实测 720×488 的启动器，
   `tt.Wheel(x, y, 0, 120)` 在 `y=20/30/45` 一行都不滚，`y=58` 才滚 1 行、`y=70` 滚 3 行。
   根因在框架：`rt.scroll` 只沿**命中链**找滚动容器（`mygo@v0.7.0/ui/input.go:489-522`
   的 `appendHitChain` 只返回最上面那个元素及其祖先），而字段行是面板里**与列表平级的
   `Absolute()` 浮层**（`launcher.go:1413`），列表是它的兄弟不是祖先，所以滚动永远找不到列表。
   `PassThrough()` 能让滚轮穿过去，但**点击也会穿过去**（会点到不透明的字段带底下那些看不见
   的行），不能这么修。`ui.ListState` 只有 `ScrollTo(row, align)` / `ScrollIntoView`，
   **没有像素级滚动**，转发只能整行跳（`wheelAnimation` 的 150ms 滑行就没了）。
   要修得先有框架侧的「滚轮改投给某个容器」或 `ListState` 的按像素滚动——属框架改动，
   写进 `.pending-bugs.md` 的仍待。
2. **拖入模式（files）的三条路径没有文字重叠**：本轮顺手用 `auditLauncherFrame` 量了
   720×488 / 720×360 / 640×360 × 深浅两外观 × en/zh，长路径与长文件名的行都在框内，
   0 条重叠、0 条出窗（侦察用的临时测试已删除，未入仓）。
3. **插件模式页（剪贴板/计算器/浏览器）在 640 宽也不重叠**：同一次侦察里把这三种页
   × 640×360 / 640×488 / 720×360 × en/zh × 深浅两外观都过了一遍 `auditLauncherFrame`，
   全绿；chips 带在 640 宽也没有出窗。
4. **`Super+0` 只落在半可见的第十行**（P11-bb 的 `last` 判据是「任何一部分在视口里」）：
   与参考 `visibleRowRange` 的 `end` 一致，且窗口高度链是红线，本轮不动。

## 8. 仍待（P11-bh）

1. 老拍板项原样有效：搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 的便条投影 / P11-bd 的「未揭示时窗口拖拽手柄接不接指针」/
   P11-be 的 `Permissions` 行「标签与胶囊同一个词」/ P11-bf 的 `configDiscardBar` 12u margin /
   P11-bg 的「顶边失败带覆盖画布顶部两行（改画布下移＝改 PTY 行数）」。
2. **新观察（需框架改动，未改）**：滚轮在字段带里不滚列表（§7 第 1 条）。要么 mygo 给
   `ListState` 一个按像素的滚动入口，要么给元素一个「滚轮转交」的开关；本仓只能等。
3. **新观察（未改，属设计决定）**：`Super+0` 落在只露一半的第十行（§7 第 4 条）。
