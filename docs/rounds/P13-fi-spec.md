# P13-fi 规格：禁用面 —— 十五个 `Disabled` 调用点，一套说法

**意图一句话**：把「这个控件现在不能用」从「十五处各自挑墨、散文里写着三种读法」变成
**一条可执行的普查**——四档材质、两种指针，多一处就红。

**为什么现在做**：`docs/liquid-glass.md` §1 的「禁用的材质」表是 P13-s / P13-t / P13-u / P13-aq
写下的，但它是一段**散文**：表里点名三种读法（面 / 槽 / 透明面），却没有一个东西守着
「新加一个 `Disabled(true)` 必须归到其中一档」。任务书把这一格列为「状态反馈的另一半」，
并给了先量再改的入口（`internal/settingsui/rows.go:568-588` 那枚建时就禁用的胶囊）。
本轮量完发现：**材质已经是一套说法**（下面有数），缺的是**把它钉住**，以及指针那一半从来没数过。

## 一、先量（真设置页地面，`dimProbe` 的四个读数）

同一行里 ON = 可用、OFF = 不可用；`crown` = 顶内线 − 面，`shade` = 底内线 − 面，`ink` = 墨/面 对比级。

| 控件 | 外观 | 面 ON → OFF | crown ON → OFF | shade ON → OFF | ink ON → OFF |
|---|---|---|---|---|---|
| 动作胶囊 `disabledActionButton` | 浅 | 230 → **238** | +6 → **+0** | −14 → −7 | 202 → **105**（0.52） |
| 同上 | 深 | 39 → **26** | +19 → **+0** | −8 → −3 | 198 → **106**（0.54） |
| accent 主 CTA `extensionPrimaryButton` | 浅 | 82 → 164 | +41 → **+0** | 0 | 168 → 43 |
| 同上 | 深 | 193 → 104 | +5 → **+0** | 0 | 170 → 40 |
| 图标格 `extensionIconButton`（透明面） | 浅 | 247 → 247 | 0 | 0 | 138 → **69**（0.50） |
| 同上 | 深 | 14 → 14 | 0 | 0 | 141 → **70**（0.50） |
| 开关 `disabledSwitch`（槽） | 浅 | 79 → 161 | +1 → +1 | 0 | 175 → 48 |
| 同上 | 深 | 187 → 103 | 0 | 0 | 68 → 77 |
| 组件卡 `uninstallPartRow`（面） | 浅 | 230 → **238** | +6 → **+0** | −14 → −7 | 202 → **105** |
| 同上 | 深 | 39 → **26** | +19 → **+0** | −8 → −3 | 198 → **106** |

读出来的三句话：

1. **面档**：禁用面离地面仍是 9 级（浅 247→238）/ 12 级（深 14→26），底对侧还在（−7 / −3），
   顶内线 **+0**——受光边熄了。这就是 §1 表里「面」那一行的算术，逐个数对得上。
2. **槽档**（开关）：内线不动（+1 / 0），整块退一半（79→161 / 187→103），没有材质可加。
3. **透明面档**（图标格）：面还是地面（247 / 14），只有墨退一半（0.50 / 0.50）。

指针那一半（`ui.Tester.Cursor()`，真 hover 六种形状）：五处 `CursorDefault`，
一处 `CursorNotAllowed`（卸载对话框里那张**锁着勾选框**的固定行卡，参考
`ComponentizedUninstallDialog.css:60-83` 的 `:has(input:disabled) { cursor: not-allowed }`）。
另有**两个站点不写 `Cursor`**（`slider` 的轨道、`detachedBar` 的键），量过：两个 widget 自己
都不设光标，解析器（`mygo@v0.7.3/ui/runtime.go:1286-1324`）落到默认箭头——所以它们不需要写，
不是漏了。**没有第三句话。**

## 二、逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | 新 `internal/settingsui/pointercensus_test.go` | 无：四档读法只在散文里 | `disabledSites` 登记 **14 个真调用点**（`file:func`）+ `TestEveryUnavailableControlIsClassified`：`internal/settingsui` 与 `internal/shell` 里每一个 `X.Disabled(...)`（含闭包里的）必须被登记，未登记即红；登记了却不再调用也红 |
| 2 | 同上 | 指针从来没数过 | `TestAnUnavailableControlPointsAtItselfWithOneOfTwoShapes`：六种形状实测，五处默认箭头 + 一处 `not-allowed`（固定行卡） |
| 3 | 同上 | 无图 | opt-in dump `TestDumpP13FIDisabledSheet`（`FLOTER_P13FI_DUMP=1`）：可用/不可用两列并排，深浅两套 |
| 4 | `docs/liquid-glass.md` §1「禁用的材质」 | 只有材质三档 | 补「指针那一半」一句 + 普查 pin 的名字，§6.130 记本轮 |
| 5 | 产品代码 | —— | **零像素改动**（量完没有一处需要改；见 §一） |

## 三、行为契约零改动红线

本轮**只加测试与文档**：高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线一个字节不动。
`uninstallPartRow` 的 `not-allowed` 是**参考里写着的**，本轮判它是**故意**（那张卡里锁着一个勾选框，
是一个表单集不是一枚键），只把它钉住，不改。

## 四、门槛命令 + PASS 基线（P13-fh 实测值）

```
gofmt -l cmd internal                       # 必须空
go vet ./...                                # 必须净
GOOS=windows go vet ./...                   # 必须净
GOOS=darwin  go vet ./...                   # 必须净
go test -count=1 ./...                      # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 基线 3406，只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

测试函数数基线 2574；SKIP 基线 190（本轮新增一个 opt-in dump 的 SKIP → 191）。

## 五、mutation 计划（≥2 条，说明每条让哪个 pin 变红）

1. **M1**：在 `internal/settingsui/rows.go` 的 `actionButton` 里加一行 `button.Disabled(true)`
   ——一个 §1 表没点名的站点。期望 `TestEveryUnavailableControlIsClassified` 红，
   报错串指名 `rows.go:actionButton` 与行号。
2. **M2**：把 `uninstallPartRow` 固定行的 `Cursor(ui.CursorNotAllowed)` 改成 `Cursor(ui.CursorDefault)`
   ——指针的两句话被并成一句。期望 `TestAnUnavailableControlPointsAtItselfWithOneOfTwoShapes` 红。
3. **M3**（顺带）：把 `disabledActionButton` 换成 `actionButton(...).Disabled(true)`
   （建好之后再禁用，冠线留着）→ 期望 `TestTheRowLockedActionPillTurnsItsCrownOff`（既有 pin）红，
   证明普查与既有材质 pin 是两层，不互相顶替。
