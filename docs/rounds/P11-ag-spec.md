# P11-ag 规格：空态的那句话必须在屏幕上

## 意图（一句话）

启动器的空态句子从来没有落在「列表的那条带」里——它被浮在卡片顶部的字段块整块盖住，
于是「一个查询什么都没匹配到」在界面上是一张**空白卡**。本轮把空态放回它的 band 里。

为什么现在做：选题顺序按用户 2026-10-28 的反馈（界面质感与交互逻辑 → bug 优先）。
F1/F2/F3 已由 P11-y/z/aa 收口；上一轮「仍待」第 1 条（`⌘K` 动作面）是拍板项；
本轮取**同一条反馈里还没被读到的一处空白**——它是界面上肉眼可见的 bug，不是内部一致性。

## 证据（本轮实测，不是旧报告的行号）

- 结构：`internal/launcher/launcher.go:1187`（改前）：`ui.Column(c).FillWidth().Padding(t.Space(3)).Center()`
  是面板的**直接子元素**，而面板是整张卡的高度；字段行是
  `launcher.fieldRow`（`launcher.go:1250` 起的绝对定位块，`.Height(fieldRow)`，
  `fieldRowUnits = 14`（56u，`internal/launcher/height.go:23`））+ 底下的 1px 发丝。
- 实测（`ui.Tester`，720×488，浅色）：空态句子矩形 `{X:334 Y:12 W:52 H:18}`；
  字段带占 y 0–56。句子整句在字段的底下面。
- 实测（同一帧的像素）：改前整帧非底像素 **562** 个，全部来自字段自己的字（占位符/查询）；
  句子矩形里一个墨点都没有 → 空白页。
- 复现命令（改前红、改后绿）：

  ```
  go test -count=1 -run TestTheEmptyPageSaysSoUnderTheField ./internal/launcher/
  ```

- 可达性：普通页的「无匹配」是一条真路径（`TestNoResultsShowsTheEmptyMessage` 专门钉了
  「不画动作条、页面说 No results」），插件模式更常走这条：剪贴板为空、浏览器没配置、
  计算器没有历史——这些页面既没有列表也没有页脚，整页只剩这句看不见的话。

## 逐条落地项

1. `internal/launcher/launcher.go` 空态（`len(results) == 0` 分支）
   - 现状：`Column.FillWidth().Padding(3u).Center()`，无 band、无高度 → 句子在卡片顶边 y=12。
   - 应改成：`Column.FillWidth().Grow(1).Center().Padding(listEdge, 3u, 3u, 3u)`——
     上边距读列表自己用的 `listEdge`（字段行 56u + 呼吸 4u + 模式的 chips band），
     `Grow(1)` 撑满面板剩下的高度，句子居中在**列表会占据的那条带**里。
     墨与字阶零改动（`Emphasis` + `TextStrong`，合约 §4.7）；无底板、无图标、无 accent 不变。
2. `internal/launcher/emptystate_test.go`（新增）
   - `TestTheEmptyPageSaysSoUnderTheField`：读**像素**（句子矩形里的墨量必须 > 0）+
     几何（句子中心 == 字段带与面板底之间的中点，±6px；且矩形顶 ≥ 字段带底）。
     旧的 `HasText` 类 pin 读的是构建出来的文本树，句子藏在字段下面它们照样绿——
     这就是它们一直没能发现这件事的原因。
   - `TestDumpP11AGEmptyPagePNGs`（opt-in `FLOTER_PNG=1`，+1 SKIP）：深/浅两帧。
3. `docs/raycast-design.md` §6.13 记本轮（§4.7 的落法在那条里点名）。

## 行为契约零改动红线

高度链（`fieldRowUnits`/`actionBarUnits`/`RowCount`）、键盘模型、会话管理、设置写回、
i18n 文案、命令接线：**零改动**。本轮只动空态这块列的布局，以及新增一个测试文件。

## 门槛命令 + PASS 基线（实测当前值）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./...                              # 必须净
GOOS=darwin  go vet ./...                              # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨（基线 2357）
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

测试函数 1922 → 1924（+2，其中 1 条是 opt-in dump）。

## mutation 计划（实跑，≥2 条）

- **M1** 把空态还原成改前的列（去掉 `Grow(1)`、上边距回到 3u）→ 句子回到 y=12：
  `TestTheEmptyPageSaysSoUnderTheField` 必须红在**墨量**那一条（证明句子确实被盖住）。
- **M2** 保留 `Grow(1)` 但去掉顶上那条 `listEdge` 内缩（`Padding(3u)`）→ 句子在面板正中
  y=244：必须红在**居中**那一条（want 268，证明 pin 钉的是「带里居中」而不是「随便可见」）。
