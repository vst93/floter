# P13-bc 规格 · accent 主 CTA 的指针阶梯：两枚同一个角色，三个不同的说法

**意图一句话**：本仓有两枚「主 CTA」（合约 §5 第 2 条点名的那一类），同一个角色，
指针落在上面时却各有各的说法——一枚**没有任何反馈**，一枚**淡掉整棵子树**；
而 `theme` 里为这件事准备好的 `AccentHover` / `AccentPressed` 两个 token，
生产代码里**零调用点**，从 `P8-a` 起一次都没画过。

**为什么现在做**：`docs/liquid-glass.md` §6 无 `未开始` 行，按纪律回通用队列的
「状态反馈」这一半。P13-q 已经把**中性**胶囊族的第三档（按下）立成契约
（`extensionActionFace` 的 `pressed` 臂），P13-al 把终端浮条的五个键补齐；
accent 族是同一句话里唯一没跟上的一族，而且它的两个 token 一直是死值——
死 token 本身就是「设计上想这么做、实现上从没做过」的证据。

---

## 1 现状（实测，`internal/settingsui/ctahover_probe_test.go` + `prof_probe_test.go`）

探针：真设置页地面（`realPageGround`，浅 247 / 深 14），同尺寸 400×160 harness，
`tt.Move` 到控件中心后跑满 120ms 颜色过渡。读数取「冠线带」「面」（盒顶 +4 行）
与「墨」（标签带里离面最远的像素）。

| 控件 | 外观 | 静息面 | hover 面 | hover 冠线 | hover 墨 |
|---|---|---|---|---|---|
| `extensionPrimaryButton`（Save / Run） | 浅 | `6/74/166` | **`6/74/166`（不动）** | 不动 | 不动 |
| 同上 | 深 | `143/183/255` | **`143/183/255`（不动）** | 不动 | 不动 |
| `updateButton`（Download） | 浅 | `6/74/166` | `29/91/173` | `66/117/187` → `29/91/173`（**冠线被淡没了**） | `247/249/255` → `226/234/247` |
| 同上 | 深 | `143/183/255` | `130/166/232` | `153/189/255` → `130/166/232` | `21/23/27` → `32/37/47` |

三条读数各自是一句话：

1. **一枚没有 hover**。`extensionPrimaryButton` 的静息帧与 hover 帧**逐像素相同**。
   一枚能按的面在指针下不动，读起来像「它没收到这个指针」。
2. **另一枚的 hover 是「淡掉整棵子树」**。`.Opacity(0.9)` 乘的是**面 + 冠线 + 墨**
   三样东西，所以 §3 那条「受光边 = 这块面可以被碰的记号」在 hover 时**自己熄了一半**——
   而 hover 恰恰是「可以碰」最确定的那一刻。
3. **同一次淡在两种外观上方向相反**。浅色档的 accent `6/74/166` 淡向 247 的地面
   变成 `29/91/173`（**变亮 = 沉进纸里**）；深色档 `143/183/255` 淡向 14 变成
   `130/166/232`（**变暗 = 也沉**）。这与 P13-r 在分段控件上判掉的「浅色档的抬起
   方向反了」是同一个算术：**用透明度表达 z，方向由脚下的地面决定，不由设计决定**。

**两个死 token**（`grep -rn "AccentHover" --include="*.go" internal/` 只命中
`theme.go:481` 的赋值与 `palette` 字段；`AccentPressed` 同）：

| token | 浅 | 深 | 调用点 |
|---|---|---|---|
| `Accent`（在用的那一档） | `#064AA6` | `#8FB7FF` | 全部 |
| `AccentHover` | `#0857C4` | `#A3C4FF` | **0** |
| `AccentPressed` | `#053C87` | `#7AA7FF` | **0** |

三档的方向是对的（浅：静息 166 → hover 196 → 按下 135，即「抬一档 / 沉一档」；
深：255 → 255（更饱和）→ 255（更暗）），而 `Control` / `ControlHover` /
`ControlPress` 早就是这条阶梯。**accent 只是没接上。**

---

## 2 逐条落地项

位置行号在动手前核过（`grep -n` 于当前 HEAD）。

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/rows.go` `extensionPrimaryButton`（`:602`） | 只有 disabled 臂；静息与 hover 同一帧 | 新增 hover / pressed 两臂：面换 `tokens.AccentHover` / `tokens.AccentPressed`，按下收 `0.97`（中性胶囊族同一个数） |
| 2 | `internal/settingsui/settingsui.go` `updateButton`（`:3929`） | `if button.Hovered() { button = button.Opacity(0.9) }` | 同上：面换 `AccentHover`，**删掉 `.Opacity(0.9)`**；补 pressed 臂 |
| 3 | 两处的共同部分 | 两个函数各自实现（已经漂成两种说法） | 抽 `a.accentPrimaryFace(c, button, disabled)`：边、面三档、冠线、按下几何、disabled 臂**一个地方**定义，两个调用点只留自己的几何 |
| 4 | `internal/theme/theme.go` | `AccentHover` / `AccentPressed` 零调用点 | 不动值，只由上面两处接上；补一条 pin 钉住「两个 token 有调用点」 |
| 5 | 普查 | — | 生产代码里 accent 面不许再出现 `.Opacity(`（那是「用透明度表达 z」，方向由地面决定） |

**几何、行为、文案零改动**：`MinHeight` / `Padding` / `Radius` / `Label` /
`Clicked()` / `Disabled` 的语义一个数不动；hover 与 press 是既有的指针手势，
本轮只补它们**画什么**。

**红线**（逐条确认不碰）：高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 /
命令接线。`updateButton` 与 `extensionPrimaryButton` 的 `Clicked()` 分支不动。

---

## 3 门槛命令 + PASS 基线（本轮实测，不是旧报告的数字）

```
gofmt -l cmd internal                                  # 必须空
go vet ./...                                           # 必须净
GOOS=windows go vet ./...                              # 必须净
GOOS=darwin  go vet ./...                              # 必须净
go test -count=1 ./...                                 # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

开工实测基线：**PASS 3122 / FAIL 0 / SKIP 103 / 测试函数 2301**。

---

## 4 mutation 计划（≥2 条，逐条说明会让哪个 pin 变红）

| # | 注入 | 预期变红的 pin |
|---|---|---|
| M1 | `extensionPrimaryButton` 的 hover 臂退回「什么都不做」 | `TestBothAccentCTAsLiftUnderThePointer`（hover 面 == 静息面 → 红） |
| M2 | `updateButton` 的 hover 臂退回 `.Opacity(0.9)` | 同上（hover 面不是 `AccentHover`）+ 冠线/墨随面动的那条（`TestAnAccentCTAsHoverMovesOnlyItsFace`） |
| M3 | 把 `AccentHover` 换成 `Accent`（两处） | 同上（面没抬 → 红）+ 死 token 普查 |
| M4 | 删掉 pressed 臂 | `TestBothAccentCTAsSinkWhenPressed`（按下 == hover → 红） |

---

## 5 视觉证据

`FLOTER_P13BC_DUMP=1` 出图 `/tmp/floter-P13-bc-accentcta-{light,dark}-{before,after}.png`：
每张图 2×3 —— 左列 `updateButton`、右列 `extensionPrimaryButton`，三行是
静息 / hover / 按下。`before` 那一组用 `FLOTER_P13BC_TAG=before` 在注入旧写法后出。
