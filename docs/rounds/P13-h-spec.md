# P13-h 规格 · 启动器的小面：28u 图标板与两处悬停小格从「四层环」退成 E1 的一对面

## 1. 意图与证据

**意图一句话**：启动器上每一行都先经过的那块 28u 图标板（以及两处悬停小格）现在戴着
**四层控件透镜**——顶受光边 + 宽 sheen + 底暗线 + **四边 `ControlEdge` 的环**；把它的
第四层（环）与第二层（宽 sheen）退掉，只留 E1 的一对面（`LitFace`：顶 `LensRim` + 底
`LensEdge`），让「面」靠面差与一条光站起来，而不是靠画框。

**为什么现在做（证据）**：

1. `internal/launcher/glyphs.go:287-291` 的注释自己写着「Raycast's plate is flat: the
   control lens is capped at 0.04 … no longer reads as a lit pane of glass」，而代码是
   `return tokens.Lens(plate)`。P12-a 把 `LensRim` 从 `0.04` 抬到 `0.09`（`docs/liquid-glass.md`
   §1 的重定基线表）、`LensEdge` 抬到 `0.06`，**注释与代码在这一轮之后反向**：这块板
   现在每行都在画一条 0.09 的受光边，外加四边环。
2. 它是这一张卡上**重复次数最多**的元素：`launcher.go:2435` 的每个结果行（启动器 /
   剪贴板 / 计算器 / 浏览器 / 文件页）与 `output.go:644` 的捕获输出页行都调它。一块
   每行都出现的「带框小方块」正是用户说的「偏 macOS 旧版本」那种工具条按钮的读法。
3. 同族的另外两处（悬停小格）是同一个语句形状：
   `launcher.go:1838` `tokens.Lens(control.Background(tokens.ControlHover))`（20u）、
   `launcher.go:3373` `tokens.Lens(e.Background(tokens.ControlHover))`（28u）。它们与
   P13-d 已经落地的 `terminalui` 悬停图标按钮是**同一处缺陷的第三个、第四处**：
   §2 的判据里「画框分层」要退掉，那一轮只修了终端页那一处。

**这不是新设计**：`docs/liquid-glass.md` §6.1 的 1-4（20u 键帽）已经钉死「启动器的小
格子 = `LitFace`」，§6.3 的 3-7 已经钉死「悬停的图标按钮 = 面不是环」。本轮把同一个
函数、同一个语汇补到这块板与这两格上，属于**把已落地的合约补齐**，不是另起一套。

## 2. 逐条落地项（行号是动手前实测的）

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| H-1 | `internal/launcher/glyphs.go:291`（`iconPlate` 的尾巴） | `return tokens.Lens(plate)`：顶 rim + 宽 sheen(0,6,10,-6) + 底 edge + 四边 `ControlEdge` 环 | `return tokens.LitFace(plate)`：顶 `LensRim` + 底 `LensEdge`，**无环、无宽 sheen**；注释重写成「E1 的一对面」并写明 P12-a 的升代把旧注释变成了假话 |
| H-2 | `internal/launcher/launcher.go:1838`（`rowControl` 的 `Hovered()` 臂） | `tokens.Lens(control.Background(tokens.ControlHover))` | `tokens.LitFace(control.Background(tokens.ControlHover))` |
| H-3 | `internal/launcher/launcher.go:3373`（`fieldSettingsButton` 的 `Hovered()` 臂） | `e = tokens.Lens(e.Background(tokens.ControlHover))` | `e = tokens.LitFace(e.Background(tokens.ControlHover))` |
| H-4 | `internal/shell/glasscensus_test.go:79-93`（两条 census 条目 `.launcher-result__icon` / `.launcher-action-bar__icon`） | 钉「`tokens.Lens(plate)` 这个调用必须还在」 | 按 P13-d 的既有机制翻成 `dropped`（「那个调用不许回来」），`test:` 指向本轮的新 pin |
| H-5 | 新增 `internal/launcher/plateface_test.go` | — | 像素 pin：板 = **面不是环**（左右两侧列与面同色）、顶内线亮于面、底内线暗于面、板外两像素仍是地（E1 无影）；深/浅两套外观 |
| H-6 | 同上文件 | — | 像素 pin：悬停的字段小格（`Clear`）左/右列与悬停面同色（旧 `Lens` 的环会让它们变一个色号） |
| H-7 | 同上文件 | — | opt-in 截图 dump（`FLOTER_PNG=1` + `FLOTER_PNG_TAG=before/after`）→ `/tmp/floter-P13-h-<surface>-<tag>-<appearance>.png` |

## 3. 行为契约零改动红线

只改**墨与面**，一行几何都不动：

- 28u 的板盒、`RadiusXS`、行内 padding / 图标列 28u / 文字列 54u / `rowUnits` 高度链：**不动**。
- 键盘模型、会话管理、设置写回、i18n 文案、命令接线：**不动**（本轮没有新增字符串）。
- 悬停的 `ControlHover` 面阶、按下 `ControlPress` + `Scale(0.92)`：**不动**（只换那圈线）。
- 浏览器行的裸格（`launcher.go:2432` 的透明 28u 盒）：**不动**——它本来就没有板。

## 4. 门槛命令与 PASS 基线（本轮开工前实测）

```
gofmt -l cmd internal                       # 必须空
go vet ./...                                # 必须净
GOOS=windows go vet ./...                   # 必须净
GOOS=darwin  go vet ./...                   # 必须净
go test -count=1 ./...                      # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

**基线（P13-h 开工前，`/tmp/p13h-baseline.log` 实测）**：PASS **2898** / FAIL **0** /
SKIP **65** / 测试函数 **2144**。门槛要求：PASS 只许涨，函数数不许减。

## 5. mutation 计划（每条会让哪个 pin 变红）

| # | 注入 | 应当变红 |
|---|---|---|
| M1 | `iconPlate` 换回 `tokens.Lens(plate)` | `TestTheIconPlateIsAFaceNotARing`（左右两侧列 != 面） |
| M2 | `LitFace` 只留顶边、删底边 | 同一条 pin 的「底内线暗于面」 |
| M3 | 给板加一笔 `Shadow(0,1,4,0,ShadowAmbient)` | 同一条 pin 的「板外两像素仍是地」 |
| M4 | `fieldSettingsButton` 悬停换回 `Lens` | `TestTheHoveredFieldControlIsAFaceNotARing` |
| M5 | `iconPlate` 的板面从 `Control` 掉到 `ControlPress`（面差掉到地板以下） | 同一条 pin 的「面阶 ≥ 地板」 |

## 6. 分类预案

- **V**：`internal/launcher/plate_lens_test.go`（`TestTheResultPlateCarriesTheStopLens`，
  若「板随玻璃档升」的差值因退掉 sheen 而跌破它自己的可见门槛，则按实测重定基线并写明
  旧值 → 新值）+ 两条 census 条目。
- **B**：其余 2140+ 条零改动必须全绿；红了是产品回归。
- **S**：预计 0。
