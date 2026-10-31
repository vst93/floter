# P13-ag 报告：「增加对比度」不该把边抹掉

规格 `docs/rounds/P13-ag-spec.md`。`docs/liquid-glass.md` §6 无 `未开始` 行，按纪律走通用
队列的「状态反馈 / 材质一致性」。

**这轮为什么让它更轻 / 更有空间感**：受光边是这一代语言里唯一一条**说「这块面在 z 上」**
的线（§3）。而它在「增加对比度」这一个设置下被乘了两道系数（档位 0.5 × 对比度 0.6），
落到 0.027——**要求把边看清楚的人，拿到的是一条看不见的边**。修好之后，深色面上那条线
在最薄的档位 + 对比度下从 6 级回到 12 级：材质最薄的时候，面仍然站得起来。

## 落地

| 改动 | 位置 |
|---|---|
| `litEdge(c, factor, contrast)`：对比度模式里 `alpha = 0.09 × max(factor, 0.75)`，`factor <= 0` 仍然 0，非对比度原样 | `internal/theme/theme.go`（`LensRim:` 一处调用 + 新函数 + `LitEdgeFloor`） |
| 三条账：对比度臂的地板、薄档仍比厚档安静、`off` 不复活 | `internal/settingsui/rimstop_test.go`（+ 一份 opt-in 出图） |
| §1 表 / §3 表 + 注 / §6.0 第 0-10 行 | `docs/liquid-glass.md` |

**实测（真 `theme.Tokens.LitFace`，`counterPane` 取样，深色面 65）**

| 深色冠线抬升 | `frosted` | `regular` |
|---|---|---|
| 普通（未动） | +10 | +19 |
| 对比度·旧 | **+6** | +12 |
| 对比度·新 | **+12** | **+13** |

浅色：对比度 `regular` +1 → **+2**（近白面只能吃一点点白，P13-n 的算术）。

**先量再改的转折（如实记）**：规格最初的假设是「`frosted` 普通档 10 级 = 画不出」。
把地板加到所有档位试了一次：普通档 10 → 19，于是 `frosted` 的总光量**超过** `regular`
（受光边一样亮、对侧暗线更淡），撞红两条既有 pin——`launcher/plate_lens_test.go:54`
（`off 915932 / frosted 916672 / regular 916654 / liquid 917082`）与
`settingsui/lens_test.go:91`。那两条 pin 守的是参考的 `--glass-lens-scale` 阶梯（0.5/1/1.6
是**真的一格**），拆掉它比修对比度那一下的损失大得多。**所以本轮只动对比度模式那一臂**，
普通档位逐像素不变——10 级是量出来的「安静」，不是缺陷。

## 偷了什么（qiaomu-design / craft-loop.md）

读了 `SKILL.md` → `references/user-preferences.md`（P-1 无斜体 / P-38 无左侧竖线 / P-26
奥卡姆、P-5 可读性优先逐条对照）→ `craft-loop.md` + `preflight.md`。走「已有方向，只要打磨」
车道（没有 Phase 2 四方向预览：方向是用户给的）。

- **工艺密度菜单第 12 项「材质细节 · 内边框高光」**：本轮改的就是这颗高光的**下限**——
  落成 `LitEdgeFloor = 0.75`（0.09 → 0.0675 的地板），也就是「薄档最多拿走四分之一」。
- **第 5 项「多层染色阴影」不适用**（受光边不是阴影；`ShadowAmbient`/`ShadowContact` 本轮零改动）；
  **第 2 项 ::selection、第 4 项签名交互**在自绘 Go UI 里各有既有实现，为凑数去加就是 A 节的
  「无语义装饰」——按删减通道**主动不做**。
- **DNA 注入**：供体是参考实现自己的 `base.css`（`--glass-lens-scale` 0.5/1/1.6 与对比度块的
  0.6）与 `glassmap.go` 里那句「sheen 是装饰性折射」。偷的三条值：`0.5 / 0.6` 两级缩放、
  「装饰的那一半才该被压」、`off` 档没有镜片。
- **两轮制**：第一轮的改法（全档地板）被「工程师 / 甲方」两视角否掉——它让档位阶梯不再是阶梯，
  且撞红两条既有 pin；第二轮只修对比度那 3-5 处里的一处（`craft-loop.md` 的「只修 3-5 处，
  禁止推倒重做」）。

## preflight 逐条

- **A 反 AI 套路**：零新增装饰层、零渐变、零 glow；地板是**减少**一条线在某个设置下消失，
  不是增加视觉元素。无斜体（P-1）、无左侧竖线（P-38）。**过**。
- **B 设计读取一致性**：读作「桌面启动器 + 设置页 · 工具型功能页」，DENSITY 6 / VARIANCE 4 /
  MOTION 3；本轮不动布局、配色、动效，只动一根 1px 线在无障碍设置下的下限。**过**。
- **C 动效抽查**：无动效改动（材质是静态墨）。**过（不适用已注明）**。
- **D2 中文页**：本轮零文案改动（`i18n` 一字未动），中文页与英文页同一条线。**过**。
- **E 三秒测试**：眯眼看深色面，对比度模式下顶边与面之间现在读得出一条界线。**过**。

## V / B / S 台账

- **V 2**：`internal/theme/theme.go`（墨的新下限）、`docs/liquid-glass.md`（§1/§3/§6.0 的数值表）。
- **B 0**：`TestLensScalesWithTheGlassStop`、`launcher/plate_lens_test.go`、
  `settingsui/lens_test.go` 的断言行**一个字没改**且全绿——这正是本轮「只动对比度那臂」的证明。
- **S 0**。

## 门槛（实测）

- `gofmt -l cmd internal` → 空
- `go vet ./...` / `GOOS=windows|darwin go vet ./...` → 净
- `go test -count=1 ./...` → 全绿；PASS **3032 → 3035**；测试函数 **2238 → 2240**；
  SKIP 88 → 89（+1 = 新出图的 opt-in）
- 三平台 `go build ./...` → 过

## 变异（3 条实跑，全红，`md5sum` 逐字节还原后 `cp` 复跑绿）

1. `litEdge` 不看 `contrast`（退回 `scaleAlpha`）→
   `rimstop_test.go:55: [dark/normal] the lit edge lifts the face by 10 levels on the 'frosted' stop` /
   `[dark/high-contrast] ... 6 levels ... (floor 12, 'regular' 12)`。
2. 删掉 `factor <= 0` 守卫 →
   `theme_test.go:564: off kept a lens: {255 255 255 17 ...} / {255 255 255 0 ...}`。
3. `LitEdgeFloor = 1.0` →
   `rimstop_test.go:65: [dark/normal] 'frosted' lifts 19, 'regular' 19: the thinnest stop has to stay quieter than the regular one, just not invisible`。

## 出图

`/tmp/floter-P13-ag-rim-{dark,light}{-before,}.png`（同一块 E1 面，普通 / 对比度两档并排）。
逐像素复核（读 PNG）：深色「对比度」那块面的冠线 **51 → 53**（抬升 +11 → **+13**），
普通那块 **58 → 58**（一个像素没动）；浅色「对比度」冠线 **231 → 232**。这就是本轮
「只动对比度那一臂」在图上的样子。

## 回收

`stash@{0}`（`afc38b247c`，10:06）与 `stash@{1}`（`114b7d1585`，09:16）已分别在 P13-af /
P13-ae 的报告里记为「已回收 / 可 drop」，本轮没有再新增抢救 stash。`git stash list` 核对过，
无未回收内容。

## 仍待（P13-ag）

1. **需拍板**：普通 `frosted` 档的受光边只有 10 级（`regular` 19）——要不要给普通档也加地板？
   本轮量下来「加了就撞红参考的档位阶梯」，所以**没加**。这一条要用户拍板：薄材质档的
   受光边是「更安静」还是「一样亮」。
2. P13-af 仍待 1/2/3、P13-ae 仍待 1 原样有效（均需拍板）。
