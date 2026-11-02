# P13-du 规格 · 夹具的「画家取整」一族：`int()` 截断 → `round(x+w) − round(x)`

轮号 **P13-du**（上一轮 P13-dt）。选题 = `P13-dt`「仍待」第 1 条，也是
`docs/liquid-glass.md` §6.89 末尾列的**最后一族夹具账**：「画家取整」。

## 1. 意图 + 为什么现在做

**一句话**：三处夹具在**取像素行/列**时用 `int()` 截断、或直接写库存尺的像素字面量，
于是它们量的盒子与 painter 真正涂的那一格差一行/一列 —— 其中两处因此变成了**假绿**。

为什么现在做：这一族是 §6.86「库存尺 → 出货尺」转换的**收尾**（§6.87/§6.88/§6.89 已交割
前两块），做完 `internal/launcher` 夹具就没有「量 app 不画的帧」的账了；之后按总目标回
`liquid-glass.md` §6 之外的通用细节队列。

## 2. 先量（临时探针 `TestZZProbeP13DU*`，出完数即删）

### 2.1 便条板 `noteplate_test.go`（`readPlate` 的盒子）

| 尺 | 板盒 | 宽 | 面差 door/note | 冠线 door/note | 底对侧 door/note | 字形差 door/note |
|---|---|---|---|---|---|---|
| 库存（`render`） | `16..43` / 28×28 | **28** | 18 / 18 | 5 / 0 | 14 / 14 | 122 / 93（drop 29） |
| 出货（3.6） | `14..39` / 25…26 | **26** | 18 / 18 | 6 / 0 | 14 / 14 | 101 / 87（drop **14**） |

深色出货：面差 24/24、冠线 19/0、底对侧 8/8、字形 101/78（drop 23）。

- 25.2 的盒子落 **26** 列：`round(16u)=14`、`round(44u)−1=39` → 26。
- ①②③ 在出货尺下**原样通过**（面差同值、冠线 6/0、底对侧 14/14）。
- ④ 的阈值 `15` 是库存尺的产物（实测 drop 29），出货尺浅色只有 **14** —— 阈值本身是
  「库存尺的魔法数」。**改成身份 pin**：门板的字形墨**正好**是 `TextMuted` 压在面上
  （实测 101 == gap(`TextMuted`,face) = 101），便条**正好**是 `TextTertiary`（87 == 87；
  深色 101/78 同样恒等）。断言更强、且不再有魔法数。

### 2.2 行块 `raycast_test.go` `rowBlockIsTheInsetRoundedPill`

出货尺实测（`renderShipped`）：`row.X=48.60` `row.Y=80.00` `row.H=19.00`，
描出的块 `left=7`、`right=712`。

| 量 | 现状（库存尺字面量） | 出货尺真值 |
|---|---|---|
| 左边沟 | `int(LightTheme().Space(2))` = **8** | `round(8u)` = **7** |
| 右边沟 | `InputWindowWidth − 8 − 1` = **711** | `round(720 − 8u) − 1` = **712** |
| 文字列 | `54u`（`× LightTheme().Spacing/4`），**带 ±1.5 容差带** | `48.60` = `54u` 整 |
| 角行 | `int(row.Y − row.H/2)` = **70**（真块顶是 `round(70.5)` = **71**） | **71** |

**两处假绿**：右边沟在出货尺下少 1 列（711 对方块涂到 712 —— 现状会红，见 §5 M2 反而不红，
因为左边沟先红）；角行读的是块**外面**一行，方形角永远也读不到。

### 2.3 剪贴板便条 `clipboardoff_test.go` `TestTheClipboardOffNoteIsChargedTheStatusBand`

`int((rowUnits − launcherStatusUnits) × a.Spacing)`：`2.5 × 4.0 = 10` 恰好整，库存尺下看不出来。
出货尺（`3.6`）实测：`note=104`、`plain=113`、差 **9**，而 `int(2.5 × 3.6)` = **8**
（`float64` 的 `8.999…` 被截断）。→ 现状在出货尺下**谎报 8**。

## 3. 逐条落地项

**红线**：只动 `internal/launcher/*_test.go`（本轮产品代码零改动）。高度链、键盘模型、
会话管理、设置写回、i18n、命令接线一律不碰。

1. `clipboardoff_test.go`
   - `tt := render(t, a)` → `renderShipped(t, a, false)`（`a.Spacing` 从 4.0 变 3.6）。
   - 期望差 `int((rowUnits−launcherStatusUnits)×a.Spacing)` → `tenths((rowUnits−launcherStatusUnits)×a.Spacing)`，
     实测差 `tenths(float64(plain−note))` —— 两边都是「十分之一像素的整数」，`int()` 截断删掉。
2. `raycast_test.go` `rowBlockIsTheInsetRoundedPill`
   - `renderIn` → `renderShipped`（出货尺帧）。
   - 左边沟 `round(8u)`；右边沟 `round(InputWindowWidth − 8u) − 1`；比较走整数像素。
   - 文字列：`54u` 用 `tenths()` 整数相等，删 `±1.5`。
   - 角行 `int(...)` → `int(math.Round(...))`。
3. `noteplate_test.go`
   - 探针换成出货尺帧（`shippedListOutput`：`c.SetTheme(a.tokens(c).Theme)` + `themedFrameHeight`）。
   - `readPlate` 的板盒断言：`28` → `left == round(16u)`、`right == round(44u) − 1`
     （`16u` = 8u 块内缩 + 8u 内边距，`44u` = +28u 图标列；`u = designUnit()`）。
   - ④ 的 `drop < 15` → **身份**：门板字形 == `gap(TextMuted over face)`，
     便条字形 == `gap(TextTertiary over face)`（深浅两套都实测恒等）。

## 4. 门槛 + PASS 基线（本轮实测）

基线（`P13-dt` 交付态，开工前实测）：PASS **3325** / FAIL 0 / SKIP **157** / 测试函数 **2490**。

```
gofmt -l cmd internal                                  # 空
go vet ./...                                           # 净
GOOS=windows go vet ./...                              # 净
GOOS=darwin  go vet ./...                              # 净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # ≥ 3325
GOOS=linux/windows/darwin go build ./...               # 三平台过
```

## 5. 变异计划（≥2，实跑变红后 `md5sum -c` 还原 + `touch` 复跑）

- **M1** `designUnit()` 返回 `spacing`（0.9 → 3.6）：便条板盒子列 + 行块沟/文字列三条一起红。
- **M2** `renderShipped` 去掉 `c.SetTheme(...)`（退回库存尺）：三处的旧值 28 / 8 / 711 / 54 原样出现 ——
  这是「转换量的是出货尺」的直接证据。
- **M3** `readPlate` 的左边沟 `round(16u)` 改成 `int(16u)`：左 14 落 14 不变（盲点照实登记），
  但右边沟 `round(44u)−1` → `int(44u)−1` = 38 变红。
