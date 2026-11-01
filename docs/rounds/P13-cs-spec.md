# P13-cs 规格 · 分隔点：全应用唯一一枚在阶梯之外的墨

**意图一句话**：`·` 这个分隔符在四个地方都画成 `TextTertiary.Alpha(0.7)`——它不是阶梯上的
一格，而是「最暗的那一格再乘 0.7」，于是它落在**阶梯之下**，而且换一张地面就换一个高度。
把四个调用点交还给 `TextTertiary` 本身，这一族就只剩阶梯。

**为什么现在做**：P13-cr（§6.61）刚立下「一个墨档要么是 token，要么是一次算术」这条判据，
并顺手量到 `launcher/output.go:794` 是同一族的第二处；那条记在 P13-cr「仍待」第 2 条上，
要求「先量它在深/浅两套外观上离它两侧那行字多少级，再决定是留在原地还是交给阶梯」。
本轮就是那次测量 + 那次决定。**证据（动手前，产品代码未改）**：

| 表面 / 地面 | 外观 | 地面 | `·` 最深像素 | 它的档位（`TextTertiary`） | `·` 占档位 |
|---|---|---|---|---|---|
| 捕获输出页条子（`output.go:794`，地面 253 / 23） | 浅 | 253 | `183,183,187`（70） | `142,142,148`（111） | 0.63 |
| 同上 | 深 | 23 | `78,78,83`（56） | `110,110,118`（89） | 0.63 |
| 设置页集成行 meta 行（`settingsui.go:2268`，面板面 229 / 39） | 浅 | 229 | `181,181,185`（48） | 事实实测 85 | 0.56 |
| 同上 | 深 | 39 | `78,78,83`（40） | 事实实测 73 | 0.55 |
| 设置页状态字的 lead dot（`settingsui.go:2159`） | 浅 | 229 | 47 | 状态字 `TextStrong` 196 | — |
| 同上 | 深 | 39 | 40 | 193 | — |

读出来三件事：

1. **它在任何表面上都低于阶梯的最后一档**（0.55–0.63 个档位），而且**越低的地面上越淡**：
   地面从 253 掉到 229，`·` 的绝对高度从 70 掉到 48。也就是说「分隔」这件事的强度**跟着
   它所在的面漂**——参考的 `.7` 是叠在一条不透明的墨上的 CSS 乘法，本仓把它接在了
   **半透明**的 token 上，于是一次乘法变成两次（token 的 alpha × 0.7）。
2. 深色档最惨：`·` 在地面 39 的面板上只有 40 级，与 `HairlineSoft` 同级——一个 2px 的
   灰点，读起来更像屏幕上的脏，而不像标点。
3. 它没有 pin。四个调用点一个像素断言都没有，所以这枚墨从 P13-af 落地至今**没有人量过**。

**落成**：四个调用点的墨从 `tokens.TextTertiary.Alpha(0.7)` 改成 `tokens.TextTertiary`。
`TextTertiary` 是阶梯的第四档（`Text > TextSecondary > TextMuted > TextTertiary`），
所以「比它引入的事实更轻」这件事仍然成立——只是现在由**档位**说，不再由一次额外的乘法说。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/launcher/output.go:794`（`outputStatusSpans`） | `Color: tokens.TextTertiary.Alpha(0.7)` | `Color: tokens.TextTertiary`；注释记 §6.62 |
| 2 | `internal/settingsui/settingsui.go:2159`（`stateMark` 的 `leadDot`） | 同上 | 同上 |
| 3 | `internal/settingsui/settingsui.go:2268`（`metaFact`） | 同上 | 同上，并改掉那段「dot 保留参考 `.7`」的注释（理由反了：`.7` 乘在 token 上是两次 alpha） |
| 4 | `internal/shell/detach.go:273`（浮窗口条） | 同上 | 同上 |
| 5 | `docs/liquid-glass.md` §4.2 墨列 + 新增 §6.62 | 墨列写着 `TextTertiary.Alpha(0.7)`（参考的 `.7`） | 墨列写 `TextTertiary`（阶梯第四档），§6.62 记 旧值 → 新值 与实测 |

**不动**：行盒、字阶（`tokens.Caption`）、字重（事实 500 / 状态字 600）、dot 与事实同属
一个 `RichText` 节点（§4.2 的折行行为）、8u+2u 的 gap、i18n 文案、命中框、任何高度链。

## 行为契约零改动

- 不碰键盘模型、会话管理、设置写回、命令接线、i18n 文案。
- `OutputView.Status` 的拼串（`runStatusParts`）不动；只换条子上那一段 span 的墨。
- `stateMark` / `metaFact` 的 span 顺序与节点划分不动（`metaline_test.go` 的五条保持绿）。

## 门槛命令 + 当前基线（实测，来自 P13-cr 收尾）

```
gofmt -l cmd internal                      空
go vet ./...                               净
GOOS=windows go vet ./...                  净
GOOS=darwin  go vet ./...                  净
go test -count=1 ./...                     全绿
PASS 3286 · FAIL 0 · SKIP 136 · 测试函数 2432
GOOS=linux/windows/darwin go build ./...    全过
```

PASS 计数只许涨；本轮新增 3 条 pin + 2 条 opt-in 出图 → 预期 PASS 3286 → 3289，
SKIP 136 → 138，测试函数 2432 → 2437。

## mutation 计划（≥2 条，每条说清哪个 pin 变红）

| # | 注入 | 预期变红的 pin |
|---|---|---|
| M1 | 把 `launcher/output.go:794` 改回 `TextTertiary.Alpha(0.7)` | `TestTheStripDrawsItsDotFromTheLegendRung`（dot 离 faded 候选更近） |
| M2 | 把 `settingsui.go:2268` 改回 alpha | `TestTheMetaDotIsTheFactRung`（同上，面板面上） |
| M3 | 把 dot 抬到 `TextMuted`（跳过一档往上） | 三条 dot pin 全部变红（离 rung 远、离 faded 也远：它比 rung 更重） |

每条都要：注入 → 看到红 → 还原 → `md5sum -c` 逐字节核对 → `touch` 再复跑一次绿。

## 出图

`/tmp/floter-P13-cs-strip-<dark|light>-<before|after>.png`（捕获输出页条子）、
`/tmp/floter-P13-cs-meta-<dark|light>-<before|after>.png`（设置页集成行）。
`before` 用「临时把墨改回 alpha → 跑 dump → 还原」取得，与 mutation 同一条路。
