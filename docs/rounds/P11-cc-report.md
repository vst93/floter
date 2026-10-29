# P11-cc 报告 · 列表底部那条带只有一行字（长句不再折到列表上）

轮号 P11-cc｜规格 `docs/rounds/P11-cc-spec.md`｜分支 `mygo-rewrite`
前 HEAD `11c2c033`（P11-cb）｜基线 PASS **2744** / 0 FAIL / 54 SKIP

## 1. 选题与证据（本轮实测，不是旧报告的数）

选题顺序走完：用户实测反馈 F1–F4 已收完 → 合约 §6 全 `已落地`（只剩拍板项）→ P11-cb
「仍待」第 1 条整张是 13 条拍板项、第 2 条是「PTY 自己画的行量不到」的尺子盲点 →
按用户定的优先级自己找下一处（界面质感与交互 / 用户没报到的 bug）。

**这是一个真的会显形的 bug**：列表底部那条带是固定 **30u** 的 `Absolute` 行
（`internal/launcher/launcher.go:3049-3050`，`Height(t.Space(7.5))`），窗口只为它长 30u
（`Geometry.Feedback` 是 `bool`，`launcher.go:1237`；`height.go:44` `launcherStatusUnits = 7.5`），
而句子的 `Text` **没有 `SingleLine()`**（`launcher.go:3091`）。句子里的字是**用户数据**——
`internal/shell/notices.go` 的 `panelFeedback` 送来 `i18n.Notifications` 拼的安装/卸载完成句，
里面带扩展名（`internal/shell/shell.go:851-968, 2036-2076, 2440-2880`）。
一个长名字就把句子折成两行。

实测（720 宽，120 个工具的列表滚到末行，HEAD 上量的）：

| 句子 | 实测矩形 |
|---|---|
| `Installed v`（短句） | `{X:41 Y:460 W:662 H:22}` —— 一行 |
| `Uninstalled ` + 4×`very-long-extension-name-` | `{X:40 Y:404 W:664 H:44}` —— **两行** |
| 列表最低行 `app-20` 的标题 | `{X:54 Y:395 W:54 H:24}`，下沿 `y=419` |

同一帧里仓库自己的「字压字」尺子当场报红（`TestScratch…` 探针，未进仓库）：

```
long toast: "app-20" {X:54 Y:395 W:54 H:24} is painted over "Uninstalled very-long-…" {X:40 Y:404 W:664 H:44} (overlap 54.0x15.0)
long toast: "app" {X:54 Y:446 W:29 H:24} is painted over "Uninstalled very-long-…" (overlap 29.0x2.0)
```

## 2. 改了什么（产品代码两行）

| 文件 | 改动 |
|---|---|
| `internal/launcher/launcher.go:3098` | `feedbackRow` 的 toast 句子加 `.SingleLine()`（框架语义：一行，超出的部分以 `…` 收口，`ui/api_gen.go:5558`） |
| `internal/launcher/power.go:154` | `confirmPowerRow` 的问句加 `.SingleLine()`——同一条带、同一个缺失、同样固定 30u 且长不高；今天 en 56 字节装得下，属于**形态补齐** |

两条注释都写清了「为什么不是折行」：要显示全文就得让带子长成两行，那是高度链
（`Geometry.Feedback` 从 `bool` 改成计数），本轮红线不碰——写进拍板候选（见 §6）。

**同族两处故意不动**：

- `internal/terminalui/terminalui.go:1336` 的失败便条：没有固定高度（`Padding` 撑高），折行
  只是「多盖几行输出」，不是「越出自己的矩形」；而终端的失败原因是用户读它才知道为什么
  没启动，截断代价更大。
- 带子的 `Height(t.Space(7.5))`：M3 实测把它改成 10 这些 pin 全绿（见 §4），说明高度链
  有没有被守住不归本轮这条 pin 管（P11-az / `TestTheStatusBandTakesItsSpaceFromTheList` 管）。

## 3. 新增 pin（`internal/launcher/statusbandline_test.go`，两条 + 一条 opt-in dump）

1. `TestTheStatusBandKeepsItsSentenceOnOneLine`（en/zh × 深/浅 四臂）：短句与长句的**字盒必须
   逐值相同**（H 和 Y 都比 0.5 严格——本轮直接比相等，`{X:40 Y:445 W:664 H:22}` 两边一致）。
   判据用的是「带子里最短那句自己的字盒」，不是字面高度：这条 pin 守的是「一句话在带子里
   永远是同一个字盒」，不是 body 阶梯的具体值。
2. `TestThePowerQuestionKeepsTheBandsLine`（en/zh）：问句的字盒 ≤ 带子的 30u（`height.go:44`
   的 7.5u × 4），且取消按钮不因它被挤出带子。**这是形态 pin，改前也是绿的**（见 §4 的 M2）。
3. `TestDumpTheStatusBandPNG`（`FLOTER_PNG=1`，`FLOTER_PNG_SUFFIX` 分开变异前后的图）：
   720×518、长句、深/浅两张。

## 4. mutation（全部实跑，逐字节还原）

| # | 注入 | 结果 |
|---|---|---|
| **M1** | `launcher.go` 的 `TextColor(tone).Grow(1).SingleLine()` → `Grow(1)`（回到 HEAD） | **红**：`the band's sentence box changed with the sentence's length: short {X:40 Y:445 W:664 H:22}, long {X:40 Y:434 W:664 H:44} — the 30u band holds one line and cannot grow`（en/zh × 深/浅 四臂全红）；`md5sum -c` 还原 ✅ |
| **M4** | `SingleLine()` → `MaxLines(2)`（等于不裁） | **红**，同上串（`long {X:40 Y:434 W:664 H:44}`）——证明 pin 量的是「折行」，不是「有没有那个调用」；还原 ✅ |
| **M2** | `power.go` 的 `.SingleLine()` 删掉 | **绿**（如实记账）：今天的问话 en 56 字节装得下一行，这条 pin 守形态不守当前宽度；还原 ✅ |
| **M3** | 带子 `Height(t.Space(7.5))` → `Space(10)`（`feedbackRow` + `heldHintRow` 两处） | **绿**：三档 pin（含 P11-az 的 `TestTheStatusBandTakesItsSpaceFromTheList`）都不动——**带子变高不归这条 pin 管**；还原 ✅ |

## 5. 视觉证据（本轮实测像素，不是「看起来还行」）

`/tmp/floter-P11-cc-band-dark.png`、`/tmp/floter-P11-cc-band-light.png`（修后）与
`…-dark-wrapped.png`、`…-light-wrapped.png`（M1 变异那一版，同一帧、同一尺寸 720×518）。
逐行数像素（PIL，x∈[40,692]）：

| | 带子的面（深 `(49,40,37)` / 浅 `(248,235,227)`） | 句子的墨 |
|---|---|---|
| 修后 | `y=441..471`（30u） | **只有 `y=460` 一行有字**（`(245,158,11)` 深 / `(217,119,6)` 浅） |
| 变异版 | `y=441..471`（**一像素没变**） | `y=450` 与 `y=470` **两行**，并且 `y=475..478` 还有 8 行琥珀色抗锯齿像素落在面**外**（深 `(91,64,21)` / 浅 `(237,194,145)`）——句子越出带子 8u，压在下面那块面板上 |

两张图逐像素 diff：变化范围 `(40,441)-(692,479)`，即**全部落在带子内 + 带子下沿外 8u**；
标题栏、搜索框、列表上半部一个像素都没动——这就是「我只把一句长话收回带子里」的证据。

「我实测看到了什么」：深色版里带子是一块 30u 的琥珀面、字只有一行居中；变异版同一块面
高度不变，但字被劈成两行，第二行贴着带子的下沿、还有 8 行像素漏到面下面去了。

## 6. 仍待（P11-cc）

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行 /
   P11-bv `AppIcons[].Label` 的 `Dark`/`Light` / P11-bw 十行预算里分组头吃一行）。
2. **新拍板项：带子里要不要显示全文**。今天的答案是「一行 + `…`」——因为带子固定 30u
   （`Geometry.Feedback` 是 `bool`）。要显示长名字得把带子做成两行，那是高度链改动
   （`bool` → 计数），牵 `height.go` 的 `feedbackBand`、列表底部 padding、`statusBand`
   三处同一个数。
3. **新发现（本轮没修，先记账）**：`tt.SetSize` 把窗口长高 30u 之后「滚到末行」的列表，
   最低行的**字盒**仍会与带子的矩形重叠 4u（`app-20` 标题 `425..449` vs 句子 `445..467`），
   副标题 `449..471` 整个落在带子里——P11-az 的 pin 用的是**电源确认**那条带（此时动作栏
   footer 让位、带子位置更低），toast 那条带上方没有同强度的断言。别把它当「本轮没修干净」，
   它和折行是两件事（折行越出 8u，这个是列表内容与 `Absolute` 带子的边界）。
4. PTY 自己画的行（20）仍然不在字盒高度尺子上（P11-cb 实测）；`barHeight` 变高变矮看不见。
5. 尺子老盲点不变：只量字盒（图标 / 面 / 描边互压看不见）；`Tester.Find` 按字符串去重，
   且**只报元素文本、不报裁切后的字形**——所以「是不是真的画了 `…`」这条 pin 看不见，
   本轮改用「字盒高度」证明（写在 pin 注释里）。

## 7. 门槛（实测）

```
gofmt -l cmd internal                # 空
go vet ./...                         # 净
GOOS=windows go vet ./...            # 净
GOOS=darwin  go vet ./...            # 净
go test -count=1 ./...               # 全绿（0 FAIL）
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 2752（基线 2744，+8）
三平台 go build ./...                # 过
```

函数数 `2043 → 2046`（`grep -rhn '^func Test' internal/ cmd/ --include=*_test.go` 实测），
SKIP 54 → **55**（新增的 opt-in dump 那一条）。
