# P13-el · 扩展页配置工具栏：写入在飞时它仍在指控

**意图一句话**：`configToolbar` 的状态词是这张卡上唯一回答「我改的东西存住了没有」的地方，
而写入在飞的那几秒里它说的是**按下 Save 之前**那句话——用户按了保存，屏幕上最安静的那格
反而更响地重复「有未保存更改」。

为什么现在做（证据）：

- `internal/settingsui/config.go:226-231`：`dirty` 只看 draft 与 baseline 的差，`dirty` 为真
  时 `status, ink = copy.ConfigUnsaved, t.Warning`。写入在飞期间 baseline 还没重设
  （`finishConfigSave` 才 `draft.saved = cloneConfigValues(...)`），所以 `dirty` 仍为真。
- 同一帧里 `configActions`（`config.go:353-360`）已经把 Save 的标签换成
  `copy.ConfigSaving`（"Saving…"）并把两颗药丸都熄灯。于是同一张卡上：**按钮说在存，
  状态词说有未保存更改**，两个控件对同一件事说两句话。
- 状态词用的是 `t.Warning`——全卡最响的一档墨。按下 Save 的瞬间，最响的那一格
  说的正是「你刚才那一下没生效」。

这是「状态反馈」通用队列里的一格（P13-ek 的「仍待」第 1 条）：**过渡态**（saving / saved /
failed）此前只有两档面，`saving` 与 `dirty` 逐字节相同。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| el-1 | `config.go:226-231` `configToolbar` 的 `status`/`ink` | `saving` 与 `dirty` 是同一对值（`ConfigUnsaved` + `Warning`） | 在飞时 `status = copy.ConfigSaving`（**复用已有 i18n 键**，不加新文案），`ink = tokens.TextTertiary` |
| el-2 | 同上，墨 | 在飞 = `Warning`（最响） | 在飞 = `TextTertiary`（与落地后的 "Saved" **同一档**）——落地那一步因此是**纯换词、零换墨**，不跳色 |
| el-3 | `internal/settingsui/configsaveword_test.go` | 无 pin 量状态词的在飞面（`configform_test.go:626` 只量按钮标签） | 新 pin：在飞时工具栏读 `ConfigSaving` 且**不再**读 `ConfigUnsaved`；墨落在 tertiary 一侧；落地后回到 `ConfigSavedState` 且**墨与在飞逐字节相同** |
| el-4 | `docs/liquid-glass.md` §6.107 | — | 记这一格与判据 |

**不改**：Save/Defaults 两颗药丸的 disabled 与标签（`configActions` 一个字不动）、
`noticeLine` 的失败带、`finishConfigSave` 的 re-baseline、任何 i18n 字符串、
工具栏的行盒与两个图标按钮（`busy` 仍只由 copy/export 驱动）。

## 行为契约零改动红线

高度链 / 键盘模型 / 会话管理 / 设置写回 / i18n 文案 / 命令接线：**全部不动**。
本轮只换「这一格画哪个词、用哪一档墨」——`ui.Text` 的调用点、行盒、`Space` 全不变。

## 门槛命令与 PASS 基线（实测当前值）

```
gofmt -l cmd internal                     # 空
go vet ./...                              # 净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...   # 净
go test -count=1 ./...                    # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 3343（基线，P13-ek）
三平台 go build ./...                     # 过
```

测试函数 2515（P13-ek 实测）→ 本轮 +1。

## mutation 计划

- **M1 退回**：删掉在飞那一段（`if a.configSaveBusy[...] { ... }`）→ 新 pin 必须在
  「在飞时仍读 `ConfigUnsaved`」与「墨仍在 warning 一侧」两处同时变红。
- **M2 只换词不换墨**：在飞只改 `status`，`ink` 仍走 `Warning` 分支 → 新 pin 的
  「墨与落地后的 Saved 逐字节相同」必须变红（证明 pin 钉的是**面**，不只是词）。
- **M3 控制组**：把在飞的墨换成 `TextMuted`（相邻一档、不是 tertiary）→ 逐字节相同的
  断言必须红（证明它不是「只要不是 warning 就绿」）。

## 出图

`ui.Tester` 导出 `/tmp/floter-P13-el-configstatus-{before,after}-{dark,light}.png`，
裁工具栏那一行（状态词 + 两个图标按钮），深浅两套。用眼睛复核：改后那一行在飞时
**不再有琥珀色**，且行盒一个像素没动。
