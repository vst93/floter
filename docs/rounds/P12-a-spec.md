# P12-a 规格 · 视觉合约升代：轻便液态玻璃 + 三档 elevation（token 层）

> 轮号：**P12-a**（新总目标「细节打磨 + 轻便液态玻璃 + 空间感」的第一轮）
> 用户原话：**「floter 项目迭代好像目前没有了大的目标，现在修改它后面的目标，整体以
> 完善应用的细节打磨。UI UX 整体参考现代化的轻便液态玻璃效果的逻辑。当前的应用整体
> UI 偏老气化，有点偏 macOS 旧版本，空间感不够。」**

## §0 意图一句话

**把「无光泽 + 靠 1px 描边分层」的那一代合约升成「一条受光边 + 三档 elevation」，
先在 token 层把「底 / 面 / 浮」的差值拉到一眼可见——这是后面每一个表面切片的尺子。**

为什么现在做（证据，全部实测于本轮开工时的 HEAD `85a7a532`）：

1. **面与底挤在一起**：`internal/theme/theme.go:763-765` 的 `control` 深 `0.075`、
   浅 `0.05`，而卡的 sheen 只有 `0.04`（`:795-797` 的 `sheenTop`）。渲一帧实测
   （`/tmp/floter-P12-a-sessions-dark-before.png`，720×580，设置页 Sessions）：
   分组卡的填充 `(49,49,51)` 对底 `(25,25,27)`——**只差 24 个亮度级**里的 24/255，
   而 1x 屏上相邻两档要 8~11 级才读得出「两层」。用户说的「所有东西挤在同一个
   z 平面上」就是这个数。
2. **「受光边」在数值上不存在**：`lensRim` 是 `0.04`（`:795`），1x 屏上一条 10/255
   的白线压在深色面上画不出「受光」，只画得出一条噪点。用户说的「有点偏 macOS
   旧版本」正是「有 1px 边但没有光」的样子。
3. **上限与目标直接冲突**：`docs/raycast-design.md` §1 最后一行写着「低光泽上限
   `LensRim`/`LensSheen`/`LensEdge`/`SheenTop` 全部 ≤ 0.04」，并解释「分层靠面 +
   线 + 字阶，不靠高光」。新目标要「有受光边」——**这两条不能同时成立**，所以
   必须先写新合约并把旧上限**显式重定基线**，否则每一轮都会有人问「这违反 §1 吗」。

## §1 落地项（逐条：位置 → 现状 → 应改成）

### 1. 新合约 `docs/liquid-glass.md`

`§0` 自绘的物理边界（面差 / 受光边 / 阴影 / 半透明四件 + 四条硬禁令）、
`§1` 材质三档与**光泽上限的旧值→新值表**、`§2` elevation 阶梯（E0/E1/E2 + 三条判据）、
`§3` 受光边与阴影数值、`§4` 圆角与间距刻度（重申不改）、`§5` 状态与动效、
`§6` **逐表面差距表**（选题队列）。

### 2. `docs/raycast-design.md` 降级

顶部改写成「已由 `liquid-glass.md` 升代，本文只保留行为契约与已落地条目」，
并**点名**被取代的两条（低光泽上限、分层手段）；§1 的 token 表同步四行新值。

### 3. token 层（`internal/theme/theme.go`）

| # | token | 旧（dark / light） | 新（dark / light） | 为什么 |
|---|---|---|---|---|
| 1 | `control` | `0.075` / `0.05` | **`0.10` / `0.07`** | 「面」要能从「底」上站起来 |
| 2 | `controlHover` | `0.105` / `0.075` | **`0.14` / `0.105`** | 抬起一档，与面的差仍是一档 |
| 3 | `lensRim` | `0.04` / `0.04` | **`0.09` / `0.09`** | 唯一的受光边；0.04 画不出来 |
| 4 | `lensEdge` | `0.04` / `0.04` | **`0.06` / `0.06`** | 受光边的对侧（顶亮一分、底暗一分） |
| 5 | `edgeHighlight` | `0.15` / `0.92` | **`0.20` / `0.92`** | 浮层顶边；浅色 0.92 不动（白压白） |
| 6 | `edgeHighlightActive` | `0.18` / `0.96` | **`0.24` / `0.96`** | 聚焦比静息亮一档 |
| 7 | `shadowAmbient` | `0.55` / `0.18` | **`0.62` / `0.22`** | 浮层的环境影 |
| 8 | `shadowContact` | `0.40` / `0.10` | **`0.44` / `0.12`** | 浮层的接触影 |

**不动**：`lensSheen`（0.04）、`sheenTop`/`sheenBottom`（0.04/0.01）、
`sheenTopActive`/`sheenBottomActive`（0.05/0.012）、`controlPress`、行态三档、
发丝线两档、accent 全家族、半径梯、高度链、字号梯。
**理由**：面**不许反光**。轻便玻璃的反光是「一条线」，不是「一片渐变」；窗口层
的大面靠透明度与外投影分层，不靠光泽。

### 4. 行为契约零改动红线

高度链（`height.go` 的 `rowUnits`/`fieldRowUnits`/`actionBarUnits`/`launcherStatusUnits`）、
键盘模型、会话管理、设置写回、i18n 文案、命令接线：**一个字都没动**。本轮只改墨与面。

## §2 门槛命令与 PASS 基线（实测于开工时）

```
gofmt -l cmd internal                        → 空
go vet ./...                                 → 净
GOOS=windows go vet ./...                    → 净
GOOS=darwin  go vet ./...                    → 净
go test -count=1 ./...                       → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   → 2801（基线 2801）
测试函数总数                                   → 2081（基线 2081）
SKIP 计数                                      → 58（基线 58）
GOOS=linux/windows/darwin go build ./...      → 三平台过
```

**PASS 计数只许涨**：本轮新增 1 条主题 pin（+2 子臂）+ 1 条合约 pin = 净 +4（实测 2805）；
函数 +3（2084）；SKIP 58 → 59（唯一新增的是本轮 opt-in 截图 dump）。

## §3 视觉 pin 处置台账（V / B / S）

| pin | 类 | 处置 |
|---|---|---|
| `internal/theme/theme_test.go` `TestTheSurfaceLadderHasAStepToSee` | **V** | 断言钉到新值（`control` 0.07/0.10、`controlHover` 0.105/0.14），注释写 `P12-a 重定基线：旧值 → 新值`；「面是面不是发丝」的地板随新面档抬 0.045 → 0.06 |
| `internal/launcher/raycast_test.go` `TestTheCardWearsTheContractsRadiusEdgeAndCast` | **V** | 「整个透镜家族 ≤ 0.04」拆成三件：面 ≤ 0.04、受光边 ≤ 0.09、**受光边严格亮于面**；`LensEdge` 单独 ≤ 0.06 且必须是无彩色暗影 |
| `internal/shell/namedrows_test.go` `TestTheSearchCardFillsTheWindowLessTheInset` | **V** | 边上那一列从「alpha ≤ 8」改成「影的形」：三通道必须为 0（填充必带色相）且 alpha < 0.2。0.18 → 0.22 的 ambient 让同一像素 8 → 10，它仍是影不是底 |
| 其余 2799 条 | **B** | **零改动全绿** |

## §4 mutation 计划（≥2 条，各说明会让哪个 pin 变红）

| # | 注入的缺陷 | 预期变红的 pin |
|---|---|---|
| M1 | `lensRim` 改回 `0.04`（受光边回到「画不出来」） | `TestTheThreeElevationRungsAreVisibleAndOrdered`（`LensRim 10` 不亮于 `LensSheen 10`）+ `TestTheCardWearsTheContractsRadiusEdgeAndCast`（同一条，两处语言各一份） |
| M2 | `control` 改回 `0.075`/`0.05`（面掉回旧台阶） | `TestTheSurfaceLadderHasAStepToSee`（两套外观各一条等值断言）+ `TestTheThreeElevationRungsAreVisibleAndOrdered`（0.07 地板） |
| M3 | 删掉 `docs/raycast-design.md` 顶部的升代声明 | `TestTheLiquidGlassContractSupersedesTheRaycastOne`（`低光泽上限` / `分层靠面` 两条 missing） |
| M4 | 把差距表某一行的状态写成 `大概做了` | 同上（「四个词之外的状态」） |

## §5 出图

`/tmp/floter-P12-a-{launcher,sessions}-{dark,light}-{before,after}.png`，
720×580，`ui.Tester`，前后各一套，深/浅两套。
