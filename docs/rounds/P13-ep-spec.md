# P13-ep 规格：动作失败那句话是**用户/插件数据**，而它坐在一个 25.2px 的行里

表面：**插件配置浮层（plugin sheet）**。合约新增 `docs/liquid-glass.md` **§6.111**。

## 意图（一句话）

浮层里动作被拒之后，那句失败说明是**插件自己写的**（`plugincfg.Field.Failed`），
而它和一枚 25.2px 的胶囊并排坐在同一行里，没有任何「一句话只有一行」的约束——
句子一长，这一行就长高，把下一组的标题顶下去。

## 为什么现在做

`P13-eo`「仍待」第 1 条指向「浮层动作行的**在飞**状态」。本轮**先量**，答案是
**没有在飞窗口**：

- `PluginSheet.Run` 的签名是 `func(key string)`，**不带回调**；
- `launcher/config.go:149` `RunConfig` 是**同步**的：`Armed` 先清、`ActionFailed` 先清，
  然后 `a.Actions.ConfigAction(...)` 当场返回 bool，拒了就写 `ActionFailed`；
- 也就是说，按下确认那一帧结束时结果已经在 state 里，下一帧画的就是最终态
  （未武装 + 失败线）。**没有「在飞」那一帧可画**，这一条按纪律记 `故意不做`
  （理由：`Run` 同步，不存在中间态；加 busy 臂要改 `Run` 签名 = 行为契约）。

于是按「先量再改」量同一行的另一半：**失败那句话本身**。
`pluginsheet.go:542` 那行 `ui.Text(c, field.Failed)` 没有 `SingleLine()`，
而它所在的行是 `ui.Row(...).Wrap()`，行高由内容决定。

对照证据（同一份代码里的先例）：启动器的停靠带为**同一类句子**加过这条约束
——`launcher.go` `feedbackRow` 的 `ui.Text(c, a.toast)....SingleLine()`，
注释写着「句子的长度是**用户数据**……不钉一行的时候，一个长名字就把句子折成两行
（实测 720 宽：`{X:40 Y:404 W:664 H:44}`），越出带子的矩形压在列表最后几行的字上」
（P11-cc）。浮层这一条是同一个病，只是没人给它钉过。

## 逐条落地项

| 位置 | 现状 | 应改成 |
|---|---|---|
| `internal/settingsui/pluginsheet.go` `sheetAction` 的失败线（`ui.Text(c, field.Failed)…`） | 只有 `FontSize(Caption)` + `TextColor(Warning)`；宽度不够时折行，行变高，浮层内容高度跟着长 | 补 `SingleLine()`（与启动器停靠带、终端便条同一族），一句话一行 |
| 其余（几何 / 字号 / 墨 / 行内顺序） | — | **一个字节不动** |

## 行为契约零改动红线

高度链（`Geometry.Height` / `Schema.Height`）、键盘模型（`configKeys`、两次按下）、
会话管理、设置写回、i18n 文案、命令接线——**全部不碰**。本轮只改一行墨的排版约束。

## 门槛命令 + PASS 基线（本轮实测，不是旧报告的数字）

```
gofmt -l cmd internal            → 空
go vet ./...                     → 净
GOOS=windows/darwin go vet ./... → 净
go test -count=1 ./...           → 全绿
PASS 计数                        → 基线 3351（P13-eo 实测），只许涨
go build 三平台                  → 过
```

## mutation 计划（≥2 条）

- **M1**：把 `sheetAction` 失败线上的 `.SingleLine()` 删掉 → `TestThePluginSheetKeepsALongActionFailureOnOneLine`
  变红（长句那一帧的 `PluginScroll.MaxY` 比短句大一整个行盒）。
- **M2**：把 `SingleLine()` 换成 `Ellipsis` 但仍允许折行（`.SingleLine()` 去掉、只留 `Ellipsis`）
  → 同上变红（同一条判据）。若两者等价，则 M2 改为把长句换成短句 → 判据的**对照臂**
  （`MaxY(short)` 自己）必须仍然 > 0，否则 pin 在空内容上通过。
- **M3**（控制组）：把失败线的字号从 `Caption` 抬到 `Body` → 两条 `MaxY` **同时**变大，
  **差值**不变 → pin 仍绿（证明 pin 量的是「差值」，不是绝对值）。
