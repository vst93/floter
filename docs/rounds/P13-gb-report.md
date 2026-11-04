# P13-gb · 报告：浮层那一档的 0.10，量的是框的**可见** alpha

- 规格：`docs/rounds/P13-gb-spec.md`
- 合约：`docs/liquid-glass.md` **§2.2**（新）+ **§6.147**（新）+ §1 表格「浮」那一行
- 选题：通用队列「**三平台一致性**」（任务书「现在该做什么」已把 `Disabled`/键盘两半收口，
  §6 差距表无 `未开始` 行；`140-5` 仍 `需拍板` → 走替代路径）

## 这轮为什么让它更轻 / 更有空间感

一层石墨里唯一的「高」是浮层：它必须比它压着的面高 0.10。本轮量出这个 0.10 在 macOS 上
被材质吃掉了 **13% / 26% / 30%**（越厚的档掉得越多），最薄的那一格只剩 linux 的 **37%**——
也就是「最漂亮的玻璃档，浮层离框最近」。修完之后一条 0.10 在三个平台、四个档位上是同一个数。

## 量（实测）

详见 §6.147。要点：

| | linux / windows | darwin `frosted` | darwin `regular` | darwin `liquid` |
|---|---|---|---|---|
| 47% 旧填充（字节） | 145 | 127 | 107 | 76 |
| 47% 旧可见抬升 | 0.302 | 0.264 | 0.223 | 0.210 |
| 47% 新填充 | 145 | 145 | 145 | 145 |
| 10% 旧 → 新填充（字节） | 51 → 51 | 26 → 56 | 26 → 82 | 26 → 112 |
| 10% 旧 → 新可见抬升 | 0.180 | 0.088 → 0.194 | 0.078 → 0.250 | **0.066 → 0.290** |

浮层的可见抬升 = `(1−C)·F`；`frosted` 10% 的框被 veil 托到 **0.12**，所以新填充是 **0.22**（字节 56）。

## 落地

1. `internal/glassmap/glassmap.go` · `Spec.FloatAlpha`：`TintAlpha + floatStep` →
   **`Composite() + floatStep`**（可见 alpha 上的一步）。`Material == 0` 时逐位不变。
   注释写清旧值 → 新值与三条实测（含「阶梯方向是反的」）。
2. `internal/glassmap/floatplatform_test.go`（新，2 函数）：三平台 × 四档 × 六滑块档的
   **字面量**表（填充字面量 + 可见抬升字面量）+ 两条不变量 + 反面臂。
3. `internal/theme/theme_test.go`：`darwin.Float.A < linux.Float.A` → **相等**
   （`P13-gb 重定基线：旧值 107 < 145 → 新值两者相等`）；`Sunken` 那条不动。
4. `internal/theme/floatlift_test.go`（新，1 pin + 1 opt-in dump）。
5. 合约：§1「浮」那一行改写、§2.2 新增（阶梯沿**平台**还站得住吗）、§6.147 新增。

**行为契约零改动**：高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n / 命令接线一字未动。
Linux 与 Windows 的渲染逐字节不变（本轮唯一改动的量在 macOS 的 veil 分支上）。

## 图（自己复核过）

`FLOTER_P13GB_DUMP=1 go test -run TestDumpP13gbFloatLift ./internal/theme/` →
`/tmp/floter-P13-gb-floatlift-dark.png` / `-light.png`。三格并排：linux 47% ｜ darwin 47% 新 ｜
darwin 47% 旧填充；浮层那块 20×10 的均值：

- 深：`(61,61,63)` ｜ `(61,61,63)` ｜ **`(84,84,86)`** —— 旧填充亮 **23 级**
- 浅：`(206,206,208)` ｜ `(206,206,208)` ｜ **`(178,178,180)`** —— 旧填充暗 **28 级**

**这张图上看到的一处变化**：新构的 darwin 与 linux 两格**逐字节相同**，旧填充那一格在深浅
两张图里都明显偏离开（浅色偏亮、深色偏淡）——「浮层比框亮多少」不再取决于窗口材质。

（窗口服务器的 veil 不在这张图里：应用不画它，headless tester 也不画。图比的是应用自己
那两笔填充，正是本轮改掉的量；veil 在前后两帧里是同一个常量。）

## 「偷了什么」（qiaomu-design）

读：`SKILL.md`（Router：已有方向 → 打磨模式）→ `references/user-preferences.md`（P-1 无斜体 /
P-5 可读性优先 / P-38 无左侧色条）→ `references/craft-loop.md`（工艺密度菜单）→
`references/preflight.md`（A / B / C / D2）。**没有**走 Phase 2 的四方向预览（方向已由用户钉死）。

| 取自 `craft-loop.md` 工艺密度菜单 | 落成的具体数值 |
|---|---|
| **#12 材质细节**（内边框高光 / glass 折射边） | 浮层填充 = **框的可见 alpha + 0.10**（0.57 → 字节 **145**）；`LensRim` 0.09（深）/ 0.24（浅）、`EdgeHighlight` 0.20 / 0.92 不动 |
| **#5 多层染色阴影**（2-3 层、拒绝单层灰影） | `ElevThrow` 两层：`0 10px 26px` 0.30×`ElevScale` + `0 1px 2px` 0.20×`ElevScale`（本轮不新增层，只保证它与抬升同一个量纲） |
| **#3 品牌 focus 环** | 已有：环在盒外 3px、`RGBA(6,74,166,0.82)` / `RGBA(122,167,255,0.78)`（P13-ga §6.146） |
| **#6 编辑细节**（hairline / 大写字距标签） | 已有：1px 发丝线按**整格**判定（P13-fu §6.140）；本轮表里的分隔行也按格写 |
| **#10 收尾彩蛋**（hover 才出现的细节） | 不适用：本轮改的是材质常数，不动交互揭示 |

## preflight 逐条

- **A 反套路**：无渐变白边（受光边是 1px 实色线，`LensRim`/`EdgeHighlight`）、无纯黑（深色浮层
  `#18191D`、地面 `17,18,20`）、无外发光、无斜体（P-1）、无三等分卡横排、无假数据。**通过**。
- **B 设计读取一致性**：本轮的记忆点一句话 = **「浮层离开框的那 0.10，在三个平台上必须是同一个
  0.10」**；一页只有一个主题（深浅两套逐值同构）；动效零改动。**通过**。
- **C 动效抽查**：本轮零动效改动（材质常数）。「动效抽样」不适用，理由写在此。**通过（N/A）**。
- **D2 中文页**：中文行高 / 字号那两条**不能过**——正文 10.8px、`Caption` 9.9px 低于
  「正文 ≥ 14px」，这是已挂了 75 份证据的 `需拍板` 项（不许擅自改）；中西文空格、全角标点由
  i18n 文案与既有 pin 守着。**部分不通过，原因见上**。

## 台账 V / B / S

- **V 1**：`internal/theme/theme_test.go` 的 `darwin.Float.A < linux.Float.A` → `!=` 相等
  （注释写旧值 107/145 → 新值 145/145）。
- **B 0**：其余测试零改动，全绿。
- **S 0**：无结构变化。

## 门槛（实测）

- `gofmt -l cmd internal` → **空**
- `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...` → **净**（三条全跑）
- `go test -count=1 ./...` → **全绿**（`./internal/ci` 的合约表检测也绿）
- PASS 计数 **3456**（基线 3453 = P13-ga 收尾实测，+3 = 两个 pin 函数 + 一个 theme pin；
  第 4 个函数是 opt-in dump，跳过计数）
- 测试函数 **2624**（基线 2620，`git grep -h "^func Test" HEAD -- internal cmd`，+4，未减）
- `GOOS=linux/windows/darwin go build ./...` → **三条都过**

## 变异（实跑，2 条）

- **M1** `FloatAlpha` 退回 `TintAlpha + floatStep`：
  `internal/glassmap` 9 格红（`darwin/liquid/10%: the float fill = 0.1, want 0.44`…
  `darwin/regular/47%: the float fill = 0.42051283, want 0.57`）+ `TestTheSameFrameYields…` 红
  + `internal/theme` 3 处红（`the darwin float = 107, want 145` / `= 26, want 82` /
  抬升 `0.07952941, want 0.2508`）。顺带把注释里的「旧值 26 / 107 / 76」验成实测值。
- **M2** `floatStep` 0.1 → 0.05：两包共 **55** 行红，含 linux 那一臂
  （`floatStep = 0.05, want the shipped 0.1`），证明这套 pin 不是「只有 macOS 会红」。
- 还原：`md5sum -c /tmp/p13gb.md5` 四条全 `成功`
  （`8a468eea…` / `5f6c2fd7…` / `4dc55e0d…` / `d36b8ba8…`）+ `touch internal/glassmap/*.go
  internal/theme/*.go` 再复跑 `ok floter/internal/glassmap 0.003s` / `ok floter/internal/theme 0.032s`。

## 仍待

1. **`140-5` 仍需拍板**（同一条顶边在深浅下读成相反的东西）：本轮没有动它。
2. 本轮新增一条**同族但未做**的账：`MaterialVeil` 只在 `FloatAlpha`/`ContentAlpha` 两处被补偿，
   而 `Sunken` 的 **0.09** 与 `Control` 的洗色是「画在 whatever 之上的面档」——它们按契约
   §2.1 第 1 条**故意不随滑块缩放**，但**没有**任何 pin 量过它们在 veil 之下是否仍与地面差
   8 级以上（macOS `liquid` + 10% 滑块是最极端的一格）。下一轮可做：**面档在 veil 之下的
   1x 可辨性**（先量后改，可能只是一条 pin）。
3. 沿用：`137-4`（About 页 92 字时标签盒 0×0）、`129-5`、`111`（浮层失败句宽度上限）、
   `138-3` / `138-4`、以及 `tokens.Caption` 9.9px / 正文 10.8px 的字号账（preflight D2）。
