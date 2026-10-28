# P11-aw 规格：插件模式页也进「一条面上没有两段字压在同一块像素上」

## 1. 意图与为什么现在做

**一句话**：把 P11-aj 立的那条不变量（一条面上没有两段字压在同一块像素上、也没有字跑出窗口）
从启动器的**三个普通页**（空页 / 有匹配 / 零匹配）扩到**三个插件模式页**（剪贴板 / 计算器 /
浏览器），并证明这把尺子不是空转的。

为什么现在做：P11-aj 的审计夹具（`internal/launcher/textframe_test.go`）只 type 普通查询，
从没走过插件模式页。而插件模式页和普通页**不共用**布局：

- 它们各自带一条筛选 chips 带（`internal/launcher/filters.go` 的 `filtersRow`，浮在字段
  行之下、列表之上，`Absolute().Top(fieldRow+Space(1))`）；
- 字段行左侧的槽里放的是 **scope mark**（模式自己的图标 + 词），不是放大镜
  （`launcher.go:1338-1355`）；
- 列表行型不同：剪贴板行有缩略图与时间戳、计算器行是「表达式 + 右侧答案」两列、
  浏览器行分 tab / 书签分组；
- 没有页脚（`actionBarFooter` 只在普通页画）。

这四处都是「文字与文字相撞」的常见现场，P11-aj 一条都没量。选题顺序：用户实测反馈
（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目（§6 全表 `已落地`，只剩 `⌘K`＝拍板）→
上一轮「仍待」第 1 条（抽屉重开的滚动偏移＝**交互决定，需拍板**）→ **界面质感与交互逻辑**
（用户 2026-10-28 定的优先档）。

## 2. 逐条落地项

### 2.1 位置

- 新文件 `internal/launcher/modetextframe_test.go`。
- 复用 `internal/launcher/textframe_test.go` 的 `auditLauncherFrame`（P11-aj 的尺子：
  两盒在两轴上都重叠 > 1 DIP 算压字，除非一盒包含另一盒；盒越出窗口算从没被看见；
  整窗宽的盒按构造跳过）。

### 2.2 现状 → 应改成

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/launcher/textframe_test.go` 的 `TestTheLauncherPaintsNoTextOverText` | 只覆盖空页 / 有匹配 / 零匹配 × 两外观，一个窗口尺寸 | **不动**（它是普通页的 pin，零改动） |
| 2 | 插件模式页 | 无任何 text-over-text / out-of-window 审计 | 新增 `TestThePluginModesPaintNoTextOverText`：剪贴板 / 计算器 / 浏览器 × 两外观 × 两个窗口尺寸（默认卡 720×488、窗口最小高 720×360）× 每个模式的空态 |
| 3 | 夹具 | 无「长到被省略号截断」的标题 | 每个模式的夹具放一条超长标题（剪贴板的长文本、计算器的长表达式 + 宽答案、浏览器的长 tab/书签标题），让文字与行尾时间戳/答案/键帽相撞 |
| 4 | 审计是否空转 | 无 | 新增 `TestThePluginModesAuditDrawsRealRows`：断言每个模式确实画出了行（不是「空列表也让审计通过」） |
| 5 | 视觉证据 | 无 | `TestDumpPluginModePNGs`（opt-in，`FLOTER_PNG=1`）导出六张 `/tmp/floter-P11-aw-<mode>-<light\|dark>.png` |

## 3. 行为契约零改动红线

- **产品代码零改动**：本轮不碰 `launcher.go` / `filters.go` / `calculator.go` / 任何视图。
  V=0 / B=0 / S=0。
- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线全部不动。
- 测试函数数只增不减：新增 3 个（`TestThePluginModesPaintNoTextOverText`、
  `TestThePluginModesAuditDrawsRealRows`、`TestDumpPluginModePNGs`）。

## 4. 门槛命令 + PASS 基线（本轮实测当前值）

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./...                              # 净
GOOS=darwin  go vet ./...                              # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

基线（P11-av HEAD `53bf541`，本轮开工前实测）：**2384 PASS** / **0 FAIL** / **29 SKIP** /
**1947 测试函数**。本轮预计 PASS **+20**（20 个子测试算进顶层？——按 `--- PASS` 计，
子测试也计数），测试函数 1947 → 1950。实际数字以本轮跑出来的为准，写进报告。

## 5. mutation 计划（≥2 条，每条说明让哪个 pin 变红）

- **M1**：`internal/launcher/filters.go:236` 把筛选提示从
  `.Grow(1).TextAlign(ui.End)` 改成 `.Absolute().Top(0).Left(0)` —— 提示字落到 chips 行的
  左上角，压在第一枚 chip 上。→ `TestThePluginModesPaintNoTextOverText` 红
  （`"All" ... is painted over "Tab to switch · Ctrl+D to favorite"`）。
- **M2**：`internal/launcher/filters.go:210` 把 chip 从 `.Focusable().Shrink(0)` 改成
  `.Focusable().Absolute().Left(0)` —— 所有 chip 叠在行首。→ 同一条 pin 红
  （`"Tab to switch ..." is painted over "All"/"Favorites"/...`）。

两条都实跑、看到红、还原、`md5sum -c` 逐字节核对、`touch` 再复跑确认绿。
