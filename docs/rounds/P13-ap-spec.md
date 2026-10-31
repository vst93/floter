# P13-ap 规格：空态的出口按它的种类取间距

**意图一句话**：空态区域里「一句话」和「一个可以按的板」不该是同一个距离——把板从「第四行字」
推到「这一段之后的一个新东西」。

**为什么现在做**：`docs/liquid-glass.md` §6 没有 `未开始` 行，按纪律走通用队列。P13-aj 落了
空态前三件东西（标记 / 句子 / 理由）的**成对间距**，§4.3 的表格里第四行是
「理由 → 动作 | 8u + 2u margin | 不变 | —」——墨上那一格是空的，没人量过。P13-ao 的「仍待 3」
也把空态/满态列成了下一格。

**证据（本轮实测，非旧报告）**：`emptyState`（`internal/settingsui/empty.go:36`）两个带动作的
调用点共用一条 `ui.Column(c).Margin(t.Space(0.5), 0, 0, 0)`（原 `:75`）：

- 会话页（`settingsui.go:1527`）的出口是 `pageButton`（`rows.go:452`）：`MinHeight(30u)` 的
  `Control` 面 + `ControlEdge` 一对 + `LitFace` + `Scale(0.97)` 按下——**一块板**。
- 自定义快捷键（`custom.go:50`）的出口是 `actionWithGlyph`：一个 `+` 记号加一句强调色文字
  ——**一行字**。

720×720、深色：标记墨带 `[153,170]`、句子 `[179,191]`（间隙 8px）、理由 `[204,211]`
（间隙 12px）、板的顶边在 **226**、板的字在 `[235,242]`。即：**理由的墨底 → 板的顶边 =
14px**，只比文字之间的 12px 多 2px。浅色同量：13px。

## 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `empty.go` 的 `emptyState` 签名 | `(c, icon, title, hint, iconSize, floor, action func())` | 加 `plate bool`（动作是板还是字），5 个调用点（2 生产 + 3 测试）各答一次 |
| 2 | `empty.go` 的动作臂 | 恒 `Margin(0.5u)` | 板：`Margin(3u)`（盒子 8u+12u=20u）；字：`Margin(0.5u)` 不变 |
| 3 | `settingsui.go:1527` | `..., t.Space(40), func()` | `..., t.Space(40), true, func()` |
| 4 | `custom.go:50` | `..., 0, 0, func()` | `..., 0, 0, false, func()` |
| 5 | `settingsui.go:1737` / `empty_test.go:33` / `integrationsempty_test.go:91` / `customempty_test.go:133` | 无动作的调用 | 补 `false` |
| 6 | 新 pin `internal/settingsui/emptyaction_test.go` | 无墨账 | 板的顶边离理由的墨 20~34px；标签仍在板内；标签盒子差 ≥22px；文字那一档 ≤16px；两档差 ≥6px；深浅两套 + opt-in 出图 |
| 7 | `docs/liquid-glass.md` §4.3 表格第四行 + 新增 §6.13 | 「不变 / —」 | 两档数字与 16-1…16-8 |

**故意的偏离**：参考的 `.settings-empty__action { margin-top: 2px }` 是给**一行字**写的（参考
那一页的出口正是文字）。板那一档抬到 12u 是本仓的判断，理由写进 §6.13 的 16-5：三对要成阶梯
8 / 12 / 20；8u 只把 14px 推到 15px，读不出来。

## 行为契约零改动红线

- 高度链：`emptyState` 的 `MinHeight(floor)`、区域 `Padding(6u)`、`Center()` 一字不动；
  无动作的调用点（integrations，`empty_test.go` 的 160u 地板账）盒子**零变化**。
- 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**零改动**（只加了一个 `bool` 参数与一个 margin）。
- 出路的形状：`pageButton` / `actionWithGlyph` 自己的面、墨、受光边、按下臂**都不动**。

## 门槛命令与 PASS 基线（本轮实测的当前值）

```
gofmt -l cmd internal          # 空
go vet ./...                   # 净
GOOS=windows go vet ./...      # 净
GOOS=darwin  go vet ./...      # 净
go test -count=1 ./...         # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 3054 → 期望 3056
GOOS=linux|windows|darwin go build ./...            # 三个都过
```
基线：PASS **3054** / FAIL 0 / SKIP **96** / 测试函数 **2260**（HEAD `6bca3b84`）。

## mutation 计划（≥2 条，实跑）

1. **M1** 板那一档退回 `t.Space(0.5)` → `TestTheEmptyRegionsWayOutStandsBelowItsSentence`
   报「14px（深）/ 13px（浅）」变红。
2. **M2** 把默认 lead 也改成 `t.Space(3)`（两档合一）→ `TestTheTextWayOutStaysOnTheParagraphsRhythm`
   报「text way out sits 21px under the why」变红。
3. **M3** 板那一档推到 `t.Space(10)`（40u）→ 同一个 pin 的上界（>34px）变红。
4. **M4** 板那一档改成 `0` → 同一个 pin 报 12px/11px 变红。

每条都要 `md5sum -c` 逐字节核对还原（本轮 `empty.go` 的 md5 = `7334b89fca4d74d58ec90183e1be85ec`），
并 `touch` 后再复跑一次，避免测试缓存。
