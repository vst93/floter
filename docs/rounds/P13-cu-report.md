# P13-cu 报告 · 健康页的「可重试」小块：它的面是父面的同一桶漆叠两次（先量，不改）

规格 `docs/rounds/P13-cu-spec.md`，合约新增 **§6.64**。**本轮不改产品代码**（`healthfaces.go`
与 HEAD 逐字节相同），交付的是这页的**第一组像素 pin** 与一条写进合约的判定。

## 做了什么

- 量清楚 `healthFailureCard` 里 `--retryable` 小块的面：卡片 `Background(RaisedWarm)`，
  小块**也是** `Background(RaisedWarm)` → 同一桶直 alpha 漆叠两次（0.13→0.169 / 0.115→0.197）。
  实测（真设置页地面）：小块对卡 **−13.7 级（浅）/ +17.3 级（深）**，卡对地面 −15.7 / +19.3。
- 把两个「命名档」候选人一起量出来：`Control` −16.3 / +22.7，`IconSurfaceHover`
  −22.7 / +31.0，`ControlHover` −24.7 / +31.0。**全都比现状响**——所以这枚 4px 的字若换上
  任何命名 token，就会比它所在的整张卡还重。结论：**故意不改**，并把这个「匿名但最安静」
  的事实钉住，免得下一轮有人「顺手换个 token」把层级翻过来。
- 健康页此前**没有任何像素 pin**；现在加了两个测试函数：面档（含候选人判别 + 「嵌入物
  不许比父面响」的不等式）与四种状态的脸（healthy / degraded / unhealthy / unknown）。

## 偷了什么（qiaomu-design，本轮只走「已有方向，只要打磨」车道）

读的是 `SKILL.md` → `references/user-preferences.md` → `craft-loop.md` + `preflight.md`
（本轮只动「层级」不动作、不排字，材质/动效/中文排版的 reference 没读，避免吃预算）。

- **`craft-loop.md` 工艺密度菜单第 2 项「z 轴只用三层，且每层必须能一句话说清为什么」** →
  落成 §6.64 的判据「嵌在卡里的面，面差必须小于卡自己相对地面的抬升」：实测阈值写成
  **−13.7 < −15.7（浅）/ +17.3 < +19.3（深）**，并做成测试里的 `got > card` 断言。
- **同菜单第 5 项「每个数值必须能被一个更大的数反驳」** → 落成候选人表：`Control` /
  `IconSurfaceHover` / `ControlHover` 三个数都被量出来并写进 pin（不是「看起来差不多」）。
- **`preflight.md` A（反 AI 套路）**：本轮没有新增任何装饰性材质、渐变或「类玻璃边」，
  唯一的产品文件是新增测试；判 **过**。
- **B（设计读取一致性）**：暖面族的两个 token（`IconSurface` 0.13/0.115、`IconSurfaceHover`
  0.21/0.20）与合约 §1 的 token 表一致，没有自造 alpha；判 **过**。
- **C（动效抽查）**：本轮零动效改动，`ReduceMotion` 下渲染；判 **过**。
- **D2（中文页）**：本轮不新增中文文案；11px `Caption` 与「正文 ≥14px」的冲突**沿旧账**
  （需拍板第 22 份证据），本轮不新增也不解决。

## 门槛（实测）

```
gofmt -l cmd internal      # 空
go vet ./...               # 净
GOOS=windows go vet ./...  # 净
GOOS=darwin  go vet ./...  # 净
go test -count=1 ./...     # 全绿
```

PASS **3290 → 3292** | FAIL **0** | SKIP **139**（不变）| 测试函数 **2441 → 2443**（+2）。
三平台 build 见提交前的门槛记录。

## 变异（2 条，实跑变红并还原）

1. **M1 · 小块的面 → `Control`**（`healthfaces.go` 第 186 行那一处）：
   ```
   light: the chip's plane = {226 214 206}, want the card's own warm plane applied twice {239 216 199}
   light: the chip's plane reads Control {226 214 206}, not the double wash {239 216 199}
   light: the chip steps 16.3 levels over its card, more than the card's own 15.7 over the page
   light: the chip's measured step = -16.3 levels, want -13.7
   dark:  the chip steps 22.7 levels over its card, more than the card's own 19.3 over the page
   ```
2. **M2 · degraded/unhealthy 的面 → `Control`**（`healthStatusFace`）：
   ```
   light/degraded:  face = {0 0 0 18}/{0 0 0 15}/{156 31 24 255}, want {214 118 44 33}/…
   light/unhealthy: 同上
   dark/degraded:   face = {255 255 255 26}/{255 255 255 18}/…, want {255 176 132 29}/…
   dark/unhealthy:  同上
   ```
   还原后 `md5sum internal/settingsui/healthfaces.go` 回到 HEAD 的值，`touch` 复跑绿。

## 出图

本轮**不改像素**，所以没有「前后对照」可出；图基线的信息在数值里（浅 `{243 230 222}` 卡 /
`{240 215 199}` 小块；深 `{41 32 29}` / `{65 48 41}`）。下一轮如果动这块面，先用
`sampleHealthFailureCard` 复现这四组像素再改。

## 仍待（P13-cu）

1. **下一格（通用队列）**：`空态满态` 与 `状态反馈` 两张表已清空，`间距节奏`/`文案密度`
   里能改的都在高度链上（红线内不能动）。下一轮建议走**边界情况**：`healthProbeRow` 的
   首行顶着一条 `Hairline` 顶线（参考的 `border-top`），与同页其它列表「首行不画线」的
   惯例相反——**先量**它到底有没有重复一道线，再决定。
2. **需拍板（沿用 + 第 22 份证据）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」。
3. **需拍板（沿用）**：扩展页空态只有一行结论、没有出路；会话页空态说错了对象；终端页
   会话抽屉/标签对失败只加字不换墨；终端外观抽屉行要不要面；`launcher/filters.go` 过滤
   chip 与 `launcher/power.go` 确认带 chip 的按下档；`ui/tooltip.go:133` 禁用元素不弹提示；
   `cardRow` 控制槽 `Shrink(0)`；多选箱宽度；路径截断保头弃尾；终端弧 S1b / S1c / S2 尾。
4. **需拍板（第 6 份证据，§6.32-4）**：同一张卡上的两种「这一层的名字」。
5. **记一笔（不改）**：`rowFace.Muted`（`.settings-row--muted` 的 0.62）仍**零调用点**。
6. **抢救 stash 核对（第二十三次）**：顶部三条（`33e03d1f6a` / `afc38b247c` / `114b7d1585`）
   已判定与 HEAD 无未接续内容，**不要再试合**。
