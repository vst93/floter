# P13-by 报告：正文与标题说的不是同一页，而那句话在说「这一页还没做完」

轮号 `P13-by`｜分支 `mygo-rewrite`｜基线 HEAD `6de76d1b`（P13-bx）
规格 `docs/rounds/P13-by-spec.md`｜合约 `docs/liquid-glass.md` §6.42 第 **42-1 / 42-2 / 42-3** 行

## 0. 一句话

设置页的正文有一个 `default` 臂，画的是「此页面将在后续迭代中到来。」；而同一帧的页头
（`a.page`）与存进 `last_settings_page` 的 id（`PageName`）都把不认识的页号夹到**第一页**。
三处读同一件事，两处说常规，一处说这一页不存在。本轮让三处读同一个夹取（新增
`bodyPage`），并把那句话从 `i18n.Copy` 里删掉——它是这个应用里唯一一句对用户承认未完成
的话，而它只被那条永远走不到的分支读过。

**这轮为什么让它更轻、更有空间感**：这一轮**不碰**墨、面、层级（§6 的材质阶梯一个字节
没动），所以它不直接增加空间感；它做的是把「一块表面上同时挂着两种说法」这种**结构噪声**
清掉——一个应用如果会在某个角上对用户说「这一页还没做完」，那么它在别处所有的精致都会被
这句话打折。顺带量到一个对下一轮有用的数（§4）。

## 1. 先量（改前）

不是像素，是**同一帧里三处文案的分歧**（行号本轮实测，见 spec §1）：

| 读者 | 函数 | 越界页号给什么 |
|---|---|---|
| 页头标题 + 提示 | `a.page(i)` 的 `default` | `PageGeneral` / `PageGeneralHint`（常规） |
| 存进设置的 id | `PageName(i)` | `pageNames[PageGeneral]`（`"general"`） |
| **正文** | `body()` 的 `switch` `default` | `copy.PagePlaceholder`（**这一页不存在**） |

为什么它永远走不到：`pageCount` = 5，五个常量全被 `body()` 覆盖；`a.Page` 只由
`selectPage(PageName(page))` 写入，而 `PageName` / `PageByName` 都夹回第一页。

## 2. 落地

`internal/settingsui/settingsui.go`：

- 新增 `bodyPage(page int) int`（紧邻 `page()`）：`page < 0 || page >= pageCount` →
  `PageGeneral`，否则恒等。**与 `page()` 的 `default` 臂、与 `PageName()` 的夹取同一个结果。**
- `body()` 的 `switch a.Page` → `switch bodyPage(a.Page)`，删掉 `default` 臂（五个实页全在，
  无 `default` 之后一个越界值不会再画一句与页头矛盾的话）。
- 注释写清这三处为什么必须同解（并记下旧 `default` 画的是什么）。

`internal/i18n/i18n.go` + `internal/i18n/copy.go`：删 `PagePlaceholder` 字段与 EN / ZH 两条。

`i18n` 的 `TestNoDeadCopy` 仍然全绿——它守的是「声明了的键必须有人读」，删掉唯一读者之后
字段必须一起走，这条本来就自洽。

## 3. 出图

**本轮无像素改动，故无出图**（不涉及墨 / 面 / 层级 / 几何）。要验的话是一条 `grep`：
`grep -rn PagePlaceholder internal/` 现在 0 命中。

## 4. 顺手量到的一件事（下一轮的选题，未改）

任务书那条「先量再改」的猜测——「设置页的内容面 `Sunken` 到底压出多少面差」——本轮量了
（`theme.ForPreferences(settings.Default(), dark, ui.Preferences{})`，默认 47% 透明度）：

| 外观 | `t.Background`（地） | `tokens.Sunken` | `FaceOver(地, Sunken)`（内容面） | 面差 |
|---|---|---|---|---|
| 深 | `14,14,16` | `17,18,20` @ α43 | `15,15,17` | **1 级** |
| 浅 | `247,247,248` | `243,244,247` @ α43 | `246,246,248` | **1 级** |

`Sunken` 是一层 0.17 的**浅色**薄罩压在近黑/近白的地上，合成之后与地只差 1 级——所以
设置页的侧栏与内容**是同一条 z 平面**，两者之间唯一的分界是那根 1px 竖发丝线
（`settingsui.go` 的 `hairlineRule(c, true)`）。这正是用户说的「所有东西挤在同一个 z 平面上，
靠 1px 描边分家」在设置页上的算术来源。

**本轮没改，因为改它要动选中页胶囊的地**：侧栏的选中行是 `RowSelected` + `LitFace` 画在
`Sunken` 上的（§6.2 第 2-2 行），把侧栏换成真面差（`Control` 档，深 0.10 / 浅 0.07 → 8~11 级）
会让胶囊与它脚下的面同时抬起来，`RowSelected` 的对比要重新定值、`liteedge_test.go` 的五条
pin 要重新基线。这是**下一轮**的事，写进了 `.pending-bugs.md`。

## 5. pin 与门槛

pin：`internal/settingsui/pagebody_test.go`

1. `TestAnUnknownPageDrawsTheSamePageItsTitleNames`——`bodyPage` 在 `[0, pageCount)` 上恒等；
   在 `-1 / -7 / pageCount / pageCount+1 / 99` 五个越界点上与 `PageName` 同解。
2. `TestNoPageSaysItIsNotBuiltYet`——两份文案表里不许再出现 `arrives in a later round` /
   `将在后续迭代中到来`（直接读 `internal/i18n/copy.go`：`TestNoDeadCopy` 守不到「有人**用**」）。

**V 0 / B 0 / S 0**：旧 pin 一条没红，测试零改动（本轮只加新文件）。

门槛（本轮实测）：

| 项 | 值 |
|---|---|
| `gofmt -l cmd internal` | 空 |
| `go vet ./...` / `GOOS=windows` / `GOOS=darwin` | 三平台净 |
| `go test -count=1 ./...` | 全绿 |
| PASS 计数 | 改前 **3220**（P13-bx 实测）→ 改后 **3222**（+2 条新函数） |
| FAIL | **0** |
| SKIP | **122**（不变） |
| 测试函数 | 2378 → **2380** |
| `GOOS=linux/windows/darwin go build ./...` | 三平台过 |

> 注：PASS 是**全量**跑出来的（`go test -count=1 -v ./...`），本轮只加 2 条测试函数，
> 所以 3220 → 3222 的 +2 与函数数 2378 → 2380 的 +2 对得上。

## 6. 变异（2 条实跑，全红，逐字节还原）

**M1**：删掉 `bodyPage` 的夹取（直接 `return page`）。

```
--- FAIL: TestAnUnknownPageDrawsTheSamePageItsTitleNames (0.00s)
panic: runtime error: index out of range [-1] [recovered, repanicked]
```

→ 证明那四行夹取是**承重**的：没有它，正文的 `switch` 拿到越界值。

**M2**：把「此页面将在后续迭代中到来。」塞回一条活着的文案（`SaveFailed`）。

```
--- FAIL: TestNoPageSaysItIsNotBuiltYet (0.00s)
    pagebody_test.go:53: i18n still ships "将在后续迭代中到来": a page of this build may not say it is not built yet
```

两条都 `cp` 备份、`md5sum -c` 逐字节核对还原（`settingsui.go` / `copy.go` 两行都报「成功」），
再 `touch` 后复跑一次（避开测试缓存）——复跑绿。

## 7. 偷了什么（qiaomu-design skill）

本轮按 `craft-loop.md` 的「已有方向，只要打磨」路径走，**不出四方向预览页、不停下来等点选**
（用户已经把方向给死：轻便液态玻璃 + 空间感）。

- 从 `craft-loop.md` 的工艺密度菜单取了 **第 2 项「文案只说一次」**（同一件事在一个表面
  上只允许有一种说法）→ 落成 `bodyPage`：页头、存 id、正文三处读**同一个**夹取，越界值在
  三处都是 `PageGeneral`。**具体数值**：`pageCount` = 5，越界点 `-1 / -7 / 5 / 6 / 99`
  五个全钉进 pin。
- 取了 **第 5 项「删掉对用户承认未完成的话」**（`user-preferences.md` P-14 的同一条精神：
  系统不要自言自语、更不要说「还没做完」）→ 落成 `PagePlaceholder` 字段 + EN/ZH 两条
  文案的删除，并用 `TestNoPageSaysItIsNotBuiltYet` 把它钉死。
- `preflight.md` 逐条过：**A（反 AI 套路）**——本轮没有新增任何装饰、渐变、玻璃边，反而是
  删；**B（设计读取一致性）**——同一表面三处文案读同一个夹取，一致性是这轮的正题；
  **C（动效抽查）**——本轮无动效改动，无新增 transition；**D2（中文页）**——删掉的那句中文
  是「此页面将在后续迭代中到来。」，它本身是**翻译腔 + 未完成声明**，删除即改善。
  不能过的项：无。

## 8. 仍待

见 `.pending-bugs.md` 的 P13-by 小节第 1 条（侧栏与内容是同一 z 平面，量到 1 级差）。
