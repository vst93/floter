# P11-bx 报告：设置页的临时状态也进「字压字 / 出窗」审计

规格 `docs/rounds/P11-bx-spec.md`。选题顺序：用户实测反馈 F1–F4 已收完 →
`docs/raycast-design.md` §6 全 `已落地`（只剩拍板项）→ P11-bw「仍待」第 1/2 条是拍板项、
第 3 条「尺子的第四处盲点」卡在框架不导出裁剪前几何（实核 mygo v0.7.3 `ui.Tester` 只有
`Find`/`Texts`/`Image`/`Click`，没有场景树）→ 按用户定的「界面质感与交互 → 基础功能 →
三平台一致性 → 像素细节」落在**界面质感**里唯一不需要拍板的一件事：把从没量过的状态量一遍。

## 1. 落地

**产品代码零改动。** 新增 `internal/settingsui/inlineframe_test.go`：

- `TestTheSettingsPagesTransientFacesPaintNoTextOverText`：四个**临时但 inline** 的状态
  × 2 语言 × 2 外观 × 2 尺寸 = **32 个子测试**，每个都过同一把 `auditSettingsFrame`
  （无重叠、无出窗）。
  - `busy re-check` —— Integrations 页，`a.repairBusy["dev.floter.broken"] = true`：
    重检 pill 换成 `busyActionButton`（转的 LoaderCircle + 同一个词）。
  - `refused recording` —— Shortcuts 页，`a.rejectedShortcut = settings.ShortcutToggleWindow`。
  - `open recorder` —— Shortcuts 页，`a.recording && a.ShortcutID == ToggleWindow`。
  - `no integrations` —— Integrations 页，`a.Integrations` 返回 nil。
- 每条臂带**自证**：帧必须与该页静止帧有像素差（差 0 判红），有自己句子的状态还要
  `HasText`（`ShortcutRejected` / `ShortcutRecording` / `IntegrationsEmpty`）。
  busy 臂的 pill 词与静止态相同，所以它只靠像素差自证。
- `FLOTER_PNG=1` 时把每臂帧写到 `/tmp/floter-P11-bx-inline-<tag>.png`（本轮实测生成了
  32 个文件，见 §5）。

**故意没并进来的一类**：save-failed 横幅与插件表单是**浮层**，横幅本来就是「盖在它所报告的
那一页上」，把它折进这把尺子等于教会尺子接受字压字。浮层归另一条规则管（P11-bg/P11-bd）。

## 2. 实测

- 32 个子测试**第一次跑就全绿**（静止帧与临时帧都无重叠、无出窗）——
  这是一条新守卫，不是一处修复。
- 审计第一次跑抓到的是**我自己的空臂**：harness 用 `Actions{}` 时 Integrations 页根本
  不画「重新检测」（`settingsui.go:2702` 要求 `Actions.RepairIntegration != nil`），
  于是 busy 臂画的是静止页。补上动作后自证转绿 —— 这条自证第一次运行就值回票价。

## 3. 台账

**V = 0 / B = 0 / S = 0**（产品代码零改动）。md5 前后逐字节相同：

```
113281e101d8e716843c77d7f2e99c3a  internal/settingsui/settingsui.go
9bc6f7c80eedcaf95dd9e6b989b032f3  internal/settingsui/rows.go
（savealert.go 只被变异临时改过，每次都 git checkout 还原，306… 原值不变）
```

门槛（本轮实测）：`gofmt -l` 空 · `go vet ./...` 净 · `GOOS=windows/darwin go vet ./...`
净 · `go test -count=1 ./...` 全绿 0 FAIL · **PASS 2562 → 2595**（+33 = 32 子测试 + 1 父测试）·
SKIP 54 不变 · 测试函数 **2028 → 2029** · 三平台 build 过。

## 4. 变异（4 条，1 条红，3 条绿并且**每条都说明了为什么**）

**M1（红，实跑）**：把尺寸表加上 `640×520`。
原始报错串（4 个子测试，en/zh × light/dark）：

```
--- FAIL: .../en_light_busy_re-check_integrations_640x520
    inlineframe_test.go:100: en light busy re-check integrations 640x520:
    the state painted the resting page byte for byte; the arm exercises nothing
```

原因：640×520 下 Integrations 的行落在折叠线以下，pill 不在画面上。已还原（尺寸表回到
`{860,900}`、`{720,560}`）。**这条同时证明自证断言不是摆设。**

**M2（绿，产品）**：`savealert.go:157` `a.rejectedShortcut == ""` → `!= ""`（拒绝句永不画）。
仍然绿，因为同一句话在**那一行下面**也画了一遍（`settingsui.go:3593` 起），
`HasText` 被行下那句满足。⇒ 这条 `want` 对「顶部横幅消失」不敏感。

**M3（绿，产品）**：`rows.go:594` busy pill `Gap(t.Space(1.25))` → `Gap(-t.Space(3))`
（spinner 压到词上）。仍然绿：`auditSettingsFrame` 只比 `tt.Texts()` 的**字盒**，
图标（spinner）与字的互压它根本看不见。

**M4（绿，产品）**：拒绝句 `FontSize(t.FontSize)` → `× 6`。仍然绿：`textframe_test.go:48`
把 `r.W >= w-1` 的**整宽文本**跳过（当初是为了不拿整宽句去比横向重叠），
而横幅那句正是整宽的 ⇒ 它从来没被量过。

## 5. 视觉证据

`FLOTER_PNG=1 go test -run 'TestDump|TestTheSettingsPagesTransientFaces' ./internal/settingsui/`
生成 32 张 `/tmp/floter-P11-bx-inline-<lang>-<appearance>-<state>-<page>-<w>x<h>.png`，
例：`/tmp/floter-P11-bx-inline-zh-dark-refused-recording-shortcuts-860x900.png`（47 KB 级）。

我在这张图上看到的一处变化（与同页静止帧相比）：Shortcuts 页在组标题之上多出一条
「该组合键已被其他应用占用」的 warning 句，色是 `t.Warning`，字重 580，落在
`tokens.Control` 的面上、`RadiusMD` 圆角内；组卡整体下移这条带的高度，两者**没有**互相压。

## 6. 仍待（P11-bx）

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行 /
   P11-bv `AppIcons[].Label` 的 `Dark`/`Light` / P11-bw 十行预算里分组头吃一行）。
2. **尺子的第五、六处盲点（本轮实测产物，不是缺陷）**：
   - 只量**字盒**：图标 / 面 / 描边与字的互压看不见（M3 绿）；
   - `r.W >= w-1` 的**整宽文本**被跳过（M4 绿）；要守整宽句，得给它单独一条「纵向必须在窗内」
     的臂，别继续加字压字臂。
3. **`want` 断言对「同一句话画两处」不敏感**（M2 绿）：要守顶部横幅，得量「横幅的盒子存在」，
   不是「这句话在屏幕上」。
4. `Element.Cache` / `#753` 脏矩形仍停在「`ui.Tester` 看不见 renderer 侧」；
   设置页 `actionSpinner` 仍是构建期转（`rows.go:580`，改法见 P11-bp §7）；
   P11-bw 第 3 条（裁剪前几何）同理卡在框架不导出场景树。
5. 老盲点不变：`Tester.Find` 按字符串去重；滚动容器里被裁掉的行不进 `Texts()`。
