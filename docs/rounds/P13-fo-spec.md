# P13-fo 规格：同一张卡上两枚 picker，宽度该来自同一条规则

**意图**：Terminal appearance 卡上有两枚 picker——字体族走 `chooseSelect`，调色板走
`paletteRow` 自己拼的一行。§6.133 把前者从字面 `Width(220)` 改成 `MinWidth(220)`（220 是
**地板**，值更长就借标签列的地），后者还留着 `Width(220)`。出厂内容看不出差别（九个调色板名
最宽 80px，都装得下），所以这是一条**规则**上的不一致，而不是像素上的病。

## 证据（先量，720×900，四台阶 × 深浅）

- `settingsui.go:4737` `paletteRow`：`ui.Select(...).Width(220)` —— 字面 px，不跟内容。
- `settingsui.go:4718` `chooseSelect`：`MinWidth(220)`（P13-fn）。
- 出厂最宽调色板名 `High contrast` 的墨：tiny 58 / small 66 / default 73 / large 80，
  盒内净宽 171.6…184.8 —— **装得下**，所以现状不红。
- 一个 55 字的标签（i18n 表之外的调用方可以传任意 `i18n.Option.Label`）在 `Width(220)` 上
  只画出 160…176px，自然墨 310…374px：被 `SingleLine` 切掉一半以上。

## 落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| 1 | `internal/settingsui/settingsui.go` `paletteRow` | `.Width(220)` | `.MinWidth(220)`，注释写清与 `chooseSelect` 同源同规则 |
| 2 | `internal/settingsui/palettefloor_test.go`（新） | 无 pin | A：九个出厂调色板名 × 四台阶 × 深浅画出同一个盒子（右列一条线）；B：长标签画满自然墨宽且盒子长大（地板不是上限） |

## 行为契约零改动红线

- 高度链：不动（`stackedRow` 的内边距、`Gap` 全不动）。
- 键盘模型 / 会话管理 / 设置写回：不动。
- i18n 文案：不动（长标签是测试里自己造的 copy 表，不碰 `i18n`）。
- 命令接线：不动。

## 门槛（本轮实测基线）

```
gofmt -l cmd internal        # 空
go vet ./...                 # 净（含 windows / darwin）
go test -count=1 ./...       # 3412 PASS / 0 FAIL / 193 SKIP / 2583 函数
```

## mutation 计划

- **M1**：`MinWidth(220)` → `Width(220)`（回到本轮之前）。预期 B 段（长标签被切）6 处红。
- **M2**：`MinWidth(220)` → `MinWidth(120)`（地板调低）。预期 B 段「盒子没长」6 处红。
