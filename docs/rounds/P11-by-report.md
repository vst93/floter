# P11-by 报告：「字压字 / 出窗」这把尺子——整宽句也下场，一份实现三处共用

规格 `docs/rounds/P11-by-spec.md`。选题顺序：用户实测反馈 F1–F4 已全部落地
（P11-y/z/aa、P11-bb/bj/bk/bo）→ `docs/raycast-design.md` §6 全 `已落地`（只剩拍板项）→
P11-bx「仍待」第 1 条整张是拍板项、第 2 条是**尺子自己的盲点**（上一轮实测产物，
写着「别继续加字压字臂，要给整宽句单独一条臂」）→ 落在这一条。

## 1. 落地

**同一把规则此前有三份拷贝**，三份都用同一句 `if r.W >= w-1 { continue }` 把整宽文本框
整类跳过：

| 文件 | 旧行 | 现状 |
| --- | --- | --- |
| `internal/launcher/textframe_test.go` | `:54` `// the panel's/list's own label, not a run` | 转发 `frameaudit.Audit(t, tag, tt, nil)` |
| `internal/settingsui/textframe_test.go` | `:48` | 同上 |
| `internal/terminalui/draweroverlap_test.go` | `:86` `// the strip's own label, the panel's, a whole row's box` | 转发 `frameaudit.Audit`，`keep` 只留条带以下 |

新增 `internal/frameaudit`（三平台可见的无标签文件，不是 `_test.go`）：

- `Read(tt)` —— 一字符串一箱子（`Tester.Find` 的索引方式，见 §5）。
- `Violations(w, h, runs)` —— **规则本身**，不碰 tester，所以能用手工盒子问它四个问题。
- `Audit(t, tag, tt, keep)` —— 帧到违规串的接线。
- `Encloses` / `Overlap` —— 包含与交叠，`internal/terminalui/drawerband_test.go:65-66`
  的手写交叠也改成 `frameaudit.Overlap`（那两行原本是第四份算术）。

新增 `internal/ci/frameaudit_contract_test.go`：源码扫描，三个表面里**不许再出现**
`w-1` 这类窗口宽度跳过，也不许再有本地 `func encloses`；且三个文件必须 import 并调用
`frameaudit.Audit`。

新增 `internal/frameaudit/frameaudit_test.go`（4 臂）：整宽 run 压在另一段字上必须被报出
（去掉包含豁免后即红）／整窗祖先仍被 `Encloses` 豁免／1 DIP 容差仍是 1 DIP／出窗必报。
**这是这把尺子的尺子**：有人把跳过放回去，第一条臂立刻红。

## 2. 实测：删掉跳过在今日三个表面上是**行为中性**的（这条很重要）

`FLOTER_WIDE=1` 临时把「整宽箱子」打出来（跑完即删，`md5sum` 已核对还原）：

```
light en match: WIDE "Launcher results" {X:0 Y:60 W:720 H:383}
light zh match: WIDE "启动器结果"     {X:0 Y:60 W:720 H:383}
light 640: WIDE "Terminal appearance" {X:0 Y:235 W:640 H:365}
light 900: WIDE "Terminal appearance" {X:0 Y:339 W:900 H:261}
（dark 与 720/1100 同形；**设置页五页一条都没有**）
```

结论有三条，都是本轮实测：

1. 三个表面今日的整宽箱子**全是祖先**（面板自己的 label、列表的 label、条带的 label），
   祖先本来就被 `Encloses` 豁免 ⇒ **删掉跳过不会让任何一帧变红**（实测：删前删后
   三个审计全绿；把旧跳过注入回 `Violations` 之后它们**仍然全绿**，见 §4 M1）。
2. 所以那句跳过是**为优化写的**（避免 O(n²) 里白算一堆必然被豁免的对），不是为正确性写的；
   它顺手扔掉的只有「整宽叶子」这一类，而这类东西在今日的产品代码里**一个都还没有**。
3. **纠正 P11-bx §4 M4 的归因**：上一轮写「拒绝句那条横幅本来就整宽，所以被跳过、
   从来没被量过」。实测**不成立** —— 设置页没有任何整宽箱子；M4（横幅字号 ×6）全绿的真实
   原因是**那一页是会重排的列**：字号变大只把下面的内容顶下去，谁也不压谁，而且被裁掉的
   run 由 `Tester.Find` 报成裁剪后的盒子，也出不了窗。⇒ 想守「字号暴涨」得靠**纵向节奏**
   的断言（字盒高度 / 行距），不是靠这把尺子。这条记进 §5 仍待，别让下一轮再按旧归因找。

## 3. 台账

**V = 0 / B = 0 / S = 0**（产品代码零改动：`git diff --stat` 只有 `internal/frameaudit/*`
（新包，只被测试引用）、三个测试文件、`internal/ci/frameaudit_contract_test.go`、docs）。
旧的三份实现一共删掉 **约 90 行重复代码**。

门槛（本轮实测）：
`gofmt -l cmd internal` 空 · `go vet ./...` 净 · `GOOS=windows/darwin go vet ./...` 净 ·
`go test -count=1 ./...` 全绿 0 FAIL · **PASS 2595 → 2601**（+6 = 4 个子测试 + 1 父测试
+ 1 条 CI 扫描）· SKIP 54 不变 · 测试函数 **2029 → 2031** · 三平台 build 过。

## 4. 变异（3 条，全部实跑变红并还原）

**M1（红，且**同时**证明删跳过是行为中性的）**：把旧跳过原样放回 `Violations`
（`kept` 只收 `r.R.W < w-1`，`frameaudit.go:75` 区）。

```
--- FAIL: TestTheFrameRuleMeasuresFullWidthRuns (0.00s)
    --- FAIL: TestTheFrameRuleMeasuresFullWidthRuns/a_full-width_run_painted_over_another (0.00s)
        frameaudit_test.go:35: a full-width run on another run's pixels went unreported: []
```

同一变异下三个**产品**审计：

```
ok  floter/internal/launcher    0.356s
ok  floter/internal/settingsui  0.616s
ok  floter/internal/terminalui  0.173s
```

⇒ 尺子的新臂是唯一能看见这类缺陷的东西；产品侧今日没有整宽叶子，所以「删跳过」本身
不出红——**如实记，不夸大**。

**M2（红，三个表面同时）**：`frameaudit.go:101` 的 `Encloses` 豁免改成 `if false && (...)`
（祖先不再豁免）。原始报错串节选：

```
textframe_test.go:75: light en match: "Launcher results" {X:0 Y:60 W:720 H:383}
    is painted over "git" {X:54 Y:60 W:20 H:24} (overlap 20.0x24.0)
... "Launcher results" is painted over "/usr/bin/git" {X:54 Y:84 W:546 H:22} (overlap 546.0x22.0)
```

（launcher / settingsui / terminalui 三个包全红。）⇒ 证明三处审计**真的**都走到这一份实现，
也证明祖先豁免是承重的——这正是当初那句跳过想挡的东西，现在由 `Encloses` 一个答案挡。

**M3（红）**：往 `internal/launcher/textframe_test.go` 末尾追加一行本地跳过
`func skipWide(w float32, r ui.Rect) bool { return r.W >= w-1 }`。

```
--- FAIL: TestTheFrameAuditHasOneRule (0.00s)
    frameaudit_contract_test.go:61: launcher/textframe_test.go:84: "func skipWide(...) bool { return r.W >= w-1 }"
        — the window-width skip is what made full-width runs invisible; leave it to frameaudit.Encloses (P11-by)
```

**还原核对**：`md5sum -c` 对 `internal/frameaudit/frameaudit.go`
（`c2d0c416a58f7790d3320b2dc2708df6`）与 `internal/settingsui/savealert.go`
（`3051418b55db36136f86cedb698a42e8`）逐字节通过；`internal/launcher/textframe_test.go`
被 M3 的 `git checkout` 一并对回过 HEAD，已重做本轮那处转发（现 `94f5794d…`），
之后 `touch` 三个包再全量复跑一次（§3 的数字是复跑后的）。

## 5. 视觉证据

**产品代码零改动 ⇒ 没有「修前 / 修后」可拍。**本轮不造图（避免拿同一帧冒充变化）；
审计所看的那些帧的图在 `docs/rounds/P11-bx-report.md` §5（32 张
`/tmp/floter-P11-bx-inline-*.png`）。

## 6. 仍待（P11-by）

1. 老拍板项原样有效（搜索框 caret / 默认浅色 3.04:1 / 终端滚动缓冲 / `⌘K` 动作面 /
   `AccentEdge` 3:1 / P11-an 便条投影 / P11-bd 拖拽手柄 / P11-be `Permissions` 用词 /
   P11-bf `configDiscardBar` 12u / P11-bg 顶边失败带 / P11-bo 空页置入终端行 /
   P11-bv `AppIcons[].Label` 的 `Dark`/`Light` / P11-bw 十行预算里分组头吃一行）。
2. **新增（本轮实测纠正）**：P11-bx M4 的归因作废 —— 「字号暴涨」这类回归**不会**被字压字
   尺子看见（重排 + 裁剪后的盒子），要单独一条**字盒高度 / 行距**的臂。别按旧归因再去找
   「整宽横幅」。
3. 尺子剩下的两处盲点不变：只量**字盒**（图标/面/描边与字的互压看不见，P11-bx M3 绿）；
   `Tester.Find` 按字符串索引（同一句话画两处只报第一处，P11-bx M2 绿）。
4. 整宽叶子的**产品级**臂还缺一个夹具：今日产品代码里没有整宽叶子，所以要守它得先造
   一个「横跨面板的横幅」夹具（不是改产品），否则只有 `frameaudit_test.go` 的
   手工盒子在守。
5. `Element.Cache` / `#753` 脏矩形仍停在「`ui.Tester` 看不见 renderer 侧」；
   设置页 `actionSpinner` 仍是构建期转（`rows.go:580`）；P11-bw 第 3 条（裁剪前几何）
   同理卡在框架不导出场景树。
