# P13-gr · 配置表单「错误 → 修改 → 恢复 → 关闭」组合状态（增量报告）

选题落在纪律第 3 档：`.pending-bugs.md` 顶部「仍待（P13-gq）」①亲自点名的下一轮首选——
先实测 `configActions` 里 `draft.clearLive()` 在恢复默认时是否连带清除**无默认值**数字的半输入。

起点 `2610de3e`（P13-gq）。合约沿用 §6.162，本轮为行为修复，无新增视觉差距行。

## 逐项

### 1. 恢复默认值连带清空无默认字段的半输入（真 bug，已修）

- **现象**：`configActions` 的「恢复默认值」分支先 `draft.clearLive()`（清**全部**字段的 live 文本），
  再只对 `field.Default != nil` 的字段写回默认值（`config.go:346-353`）。于是**无默认值**字段——
  例如用户正打到一半的 number `"1e"`——它的 `values` 一动不动（仍是解析失败留下的 `nil`），
  但 live 文本被整体抹掉，下一帧显示回落到 `nil` 渲染成的空串。**用户正在输入的字凭空消失**，
  看起来像「恢复默认」把这个字段也重置了，而按参照实现它本不该被碰
  （`ExtensionsPanel.tsx:2013-2016`：只写回声明了默认值的字段，其余原样保留）。
- **实测**（改前，探针）：`before restore: live=map[retries:1e] values=map[retries:<nil> size:2]` →
  `after restore: live=map[] values=map[retries:<nil> size:5]`。`size`（有默认）正确恢复到 5，
  但 `retries`（无默认）的 live 被抹成空 —— 值没动、显示丢了。
- **改动**：新增 `configDraft.clearLiveField(field)`（只删一个 key），恢复默认分支改用它、
  并移进 `if field.Default != nil` 内；`clearLive()`（清全部）仍留给 `finishConfigSave`
  ——落地的写入确实在字段背后移动**每一个**值，那里整体清是对的。
- **改后实测**：`after restore: live=map[retries:1e] values=map[retries:<nil> size:5]`。
  `retries` 保住 `1e`，`size` 恢复 5 且不留陈旧 live。
- **验证**：`internal/settingsui/configboundary_test.go` 新增 `TestConfigDefaultsKeepsANoDefaultFieldHalfTyped`
  （钉：无默认字段 live == "1e"、值 == nil；有默认字段值 == 5、live 无陈旧项；再对有默认字段
  setLive("7") 后二次恢复仍清掉）。既有 `TestConfigDefaultsClearOnlyRestoredFieldErrors` 不受影响。
- **台账**：V 0 / B 0 / S 0（纯行为修复，无断言重定基线）。
