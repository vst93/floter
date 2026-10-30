# P13-n 报告 · E1 是一对线：量了受光边，改了对侧

**轮号** P13-n ｜ **规格** `docs/rounds/P13-n-spec.md` ｜ **合约** `docs/liquid-glass.md` §1 的 0-8 行 / §3

## 这轮为什么让它更有空间感

深色档的 E1 块一直只有「顶亮」一半：面 39 上一条 0.06 的黑线只降 **2 级**——那不是影，
是同一个像素。把对侧抬到 0.20 之后，同一块面降 **8 级**（面 65 上是 −4 → −13），
「顶亮一分、底暗一分」第一次在深色档同时成立：块有了下边界，才读得出是「放在底上的面」，
而不是底上一条被点亮的高光带。

## 做了什么

1. `internal/theme/theme.go`：深色调色板 `lensEdge` **0.06 → 0.20**（浅色档两套 token 全部不动）。
2. `internal/launcher/raycast_test.go`：对侧上限按外观分档（浅 ≤ 0.06 / 深 ≤ 0.20），注释写
   `P13-n 重定基线`。
3. `docs/liquid-glass.md`：§1 的光泽上限表与「新上限一句话」拆深/浅两值；§3 新增 `LensEdge`
   一行；§6.0 新增 **0-8**（`已落地`）；§6.2 的 2-5 补一句凹槽随之加深。
4. 新增 `internal/settingsui/counterside_test.go`：一条像素 pin + 一条 opt-in dump。
   pin 量的是**真的** `theme.Tokens.LitFace`（不是自己画两条参照线）——所以 M3 能抓到
   「`LitFace` 的底边换成 `LensRim`」这类改动。

## 先量后改：被否掉的那个改法（本轮最重要的结论）

任务书点名要量「浅色 `LensRim` 0.09 是否真看得见」。量了：**看不见**——真设置页的面是
230，0.09 的白只抬 **+2 级**（深色档同一条线抬 +19 级，因为它的面是 39）。

于是按 §3 末尾「浅色的受光边只能是近白」把浅色抬到 0.55 试了一遍，实测面 230 的冠线
231 → 244（+2 → **+14**），中性面上确实站起来了。**但套件立刻红了**：

```
--- FAIL: TestThePrimaryCTAIsAnAccentButtonAtRadiusSM/light
    no accent ink found in the button's top row
```

那条 pin 靠「强调色 CTA 的**顶行**里能找到强调色」来量圆角（radius 8 与 stadium 的差别就
在顶行内缩多少像素）。0.55 的白把顶行洗成浅白，强调色在那一行**不存在**了——这恰好是
`docs/liquid-glass.md` §1 禁令 3 点名的「渐变白边模拟高光 / 类玻璃边」。而且浅色档并不缺
这半条线：它的对侧降 **14 级**，块靠对侧站起来。

所以这一轮**不改浅色**，改深色。这条结论连同实测表一起写进合约 0-8 行，避免下一轮有人
再把同一个改法试一遍。

## 数字（实测，真设置页地面）

| 外观 | 地面 | 面 | 冠线 | 抬高 | 对侧 | 压暗 |
|---|---|---|---|---|---|---|
| light | 247 | 230 | 232 | +2 | 216 | −14 |
| dark | 14 | 39 | 58 | +19 | 31（旧 37） | **−8**（旧 −2） |

## 门槛（全部实测）

- `gofmt -l cmd internal`：空
- `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...`：净
- `go test -count=1 ./...`：全绿
- `--- PASS` 计数 **2944**（基线 2941，+3）｜ `--- FAIL` **0** ｜ `--- SKIP` **72**（基线 71，+1 opt-in dump）
- 测试函数 **2167**（基线 2165，+2）
- `GOOS=linux/windows/darwin go build ./...`：过
- V/B/S 台账：**V 1**（`launcher/raycast_test.go` 的对侧上限档）｜ **B 0** ｜ **S 0**

## 变异（4 条实跑，全部变红并逐字节还原）

| # | 注入 | 原始报错串（截断） |
|---|---|---|
| M1 | 深色 `lensEdge` → 0.06 | `[dark] the counter-side drops its face by only 2 levels ({37 37 38} under {39 39 40} on the page ground {14 14 16})` |
| M2 | 深色 `lensEdge` → 0.60 | `LensEdge is alpha 153, above this appearance's counter-side cap 51` |
| M3 | `LitFace` 底边 → `LensRim` | `[light] the counter-side ({230 230 230}) is not darker than the face ({230 230 230}): E1 is a pair (§3)`（深色同样红） |
| M4 | `LitFace` 顶边 → `ControlEdge` | `the action pill: the rows inside its top frame are {115}, not brighter than its face {119}` + `the chosen row's top edge {224 224 225} is not brighter than its own face {238 238 239}` |

每条都 `cp` 备份 → `sed` 注入 → `touch` 后复跑（避开测试缓存）→ `cp` 还原 → `md5sum -c` 逐字节
核对（三次都是 `internal/theme/theme.go: 成功`）。

M4 的教训：新 pin 抓不到它（pin 只要求「两半里至少一半 ≥ 10 级」，顶边被换成暗线时对侧
仍然降 14 级），但**既有的** `crownOf` / `liteBrighter` 族抓到了——两族互补，不重叠。

## 出图（自己复核）

`/tmp/floter-P13-n-counter-light.png` / `-dark.png`：同一块参照面在真页面地面上画两遍
（before 0.06 / after 0.20）。

**这张图上看到的一处变化**（读的是 PNG 像素，不是印象）：深色那张的下边框内侧那一行，
before 是 37、after 是 31——比面 39 深 8 级；而浅色那张两行完全一样（216 / 216），
因为浅色档的 `lensEdge` 这一轮一个字节没动。

## 用了 `qiaomu-design`（偷了什么）

路径：`SKILL.md` → `references/user-preferences.md` → `craft-loop.md` + `preflight.md`
（「已有方向，只要打磨」那条；没有走 Phase 2 的四方向预览——用户已把方向给死）。

**从 `craft-loop.md` 的工艺密度菜单偷了什么（逐条落到数值）**：

1. **第 5 项「多层染色阴影，拒绝单层灰」** → 取它的**分层**思路而不是它的影：E1 的一对线
   是「一层亮 + 一层暗」，这一轮把暗的那一层从 −2 级做到 −8 级，让两层在深色档都读得出。
2. **第 12 项「材质细节：内边框高光」** → 顶边保持 `LensRim`（E1 唯一允许亮的那条线），
   **不加**第二条亮线：`LensEdge` 是它的对侧，不是第二层高光。
3. **第 6 项「编辑细节：hairline」** → 一个 token、一个 alpha，几何（半径/行高/边框宽度）
   零改动：`gofmt` 之后 `git diff` 里只有颜色常量与注释。

**`preflight.md` 逐条结论**：

- **A（反 AI 套路）**：过。没有渐变白边（这一轮恰恰是**否掉**了那个会把 CTA 顶行洗白的
  改法）、没有模糊贴图、没有 glass 叠 glass、没有新增阴影。
- **B（设计读取一致性）**：过。读取为「常驻浮层面板 → 设置页 → 轻便液态玻璃」；改动落在
  既有 token（`LensEdge`），没有新数值族。
- **C（动效抽查）**：过（本轮不动效）。`Transition` 一条没改。
- **D2（中文页）**：过。零文案改动、零新字符串、零斜体。

## 仍待（P13-n，追加在 P13-m 之后）

1. **浅色受光边：已量、已判、不改（结案）**。面 230 上 0.09 只抬 +2 级，但抬它会把
   accent CTA 的顶行洗白（§1 禁令 3），而浅色档靠对侧 −14 级站起来。**下一轮不必再试。**
2. **深色 `LensInset` 的顶内线随 0.20 加深了 −8 级**（凹槽：开关轨道 / 分段控件）：方向正确，
   但**没有像素 pin 单独量过**——下一轮候选（「凹槽的一对线与面的一对线不是同一对」）。
3. **`Float` 档固定 +0.10**（P13-k 第 2 条原样有效，需拍板）：`FloatAlpha()` 在 10% 框上是
   0.20 对面 0.10、在 100% 框上与框同值（`min(1,1.1)`）。
4. **§6.2b 9-11 仍是 `故意不做`**：`control.SegmentedIndex` 选中段的 accent 四边环，需拍板。
5. **P13-k 第 1 条原样有效（需拍板）**：`glass.ScrollEdge` 回放不透明底色。
6. **P13-c 四条原样有效**：S1b（`CloseAll` 改 detach，等拍板）、S1c（Windows ConPTY）、
   客户端不告而别的检测、capture 通道迁移。
7. **P13-h 的观察原样有效（需拍板）**：`iconPlate` 暖面戴 `LitFace` 时深色档那条 0.09 白线
   读不出来——本轮之后它的**对侧**更清楚了（−8），但顶边仍是 0.09（深色档的顶边在冷面上
   抬 +19、在暖面上被色相吃掉）。
