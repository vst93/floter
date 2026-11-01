# P13-ca 规格：锁定的字段还戴着半盏灯

## 意图（一句话）

设置页的锁定表单里，**字段**（`fieldFace` 画的 E1 面）在禁用时仍然戴着受光边，而同一张表单上
锁定的**胶囊**早就把灯关了——同一个应用对「你不能碰这块面」给了两个答案。这一轮让字段也熄灯，
只留下底对侧那道暗线（凹槽还在，灯灭了）。

为什么现在做：`docs/liquid-glass.md` §6 差距表已无 `未开始` 行，选题回通用队列
「状态反馈」的另一半——**禁用 / 只读面**。这一格是那一族最后一个没走 `DimFace` 的成员。

## 证据（改前，行号本轮实测）

1. `internal/settingsui/textinput.go:74` `fieldFace` = `tokens.LitFace(e.Background(tokens.Control).Border(1, tokens.ControlEdge))`
   ——**冠线 `LensRim` + 底对侧 `LensEdge`**，这就是「这块面可以被碰」的记号（§3）。
2. `internal/settingsui/config.go:169` `lockedControl` 只做 `Disabled(true).Cursor(CursorDefault)`；
   它自己的文档写着「**面**再叠 `DimFace`（熄灯），**槽**与**透明面**什么都不叠」——**规则写了，字段没照做**。
3. 三个字段的锁定臂都只调 `lockedControl`，没有 `DimFace`：
   - `config.go:563` 下拉（`configSelect` → `fieldFace`）
   - `config.go:656` 数字字段（`extensionField` → `textInput` → `fieldFace`）
   - `config.go:710` 文本字段（同上）
4. 对照：同一张表单上锁定的**胶囊**走 `DimFace`（`rows.go:604`、`settingsui.go:6071`），
   锁定的**透明面小格**什么都不叠（`rows.go:1004`）——三档都对，只有字段漏了。

## 逐条落地项

| 位置 | 现状 | 应改成 |
|---|---|---|
| `config.go` `lockedControl` 之后 | 没有「锁定的面」这一档 | 新增 `(*App).lockedField(c, e)`：`DimFace(lockedControl(e))`，一处回答「锁定的面 = 一道淡 + 熄灯」 |
| `config.go:563` 下拉 | `e = lockedControl(e)` | `e = a.lockedField(c, e)` |
| `config.go:656` 数字字段 | 同上 | 同上 |
| `config.go:710` 文本字段 | 同上 | 同上 |
| `internal/settingsui/lockedform_test.go` `assertOneFade` | 把锁定后的冠线钉成 `halfMix(rest.crown, ground)`（半盏灯） | 新增 `crownLit` 参数：字段臂传 `false`，断言**锁定后的冠线 = 锁定后的面**（0 级）；开关 / 列表框两臂仍传 `true`（它们各自的材质，不在本轮范围） |
| 新增 pin | 无 | `TestTheLockedFieldPutsTheLightOut`：① 锁定后冠线 = 面（0 级）；② 锁定后底对侧**仍在面之下**（凹槽没被抹平）；③ 静息字段的冠线**仍亮于**面（没有连静息一起关掉） |

## 行为契约零改动红线

- 只改墨：`InsetShadow` 的两条线，不碰 `Padding` / `MinHeight` / `Radius` / `Disabled` 标志 /
  `Cursor` / 命中 / 焦点链 / 写回 / i18n 文案 / 高度链。
- 锁定语义（`Disabled` 一道淡）不动：`lockedControl` 一个字节不改。

## 门槛命令 + PASS 基线（本轮实测）

```
gofmt -l cmd internal                     # 必须空
go vet ./...                              # 必须净
GOOS=windows go vet ./... ; GOOS=darwin go vet ./...   # 必须净
go test -count=1 ./...                    # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      # 只许涨
GOOS=linux/windows/darwin go build ./...  # 三平台过
```

基线（P13-bz 实测）：PASS **3225** / FAIL **0** / SKIP **123** / 测试函数 **2382**。

## mutation 计划（≥2 条，每条说明哪个 pin 会红）

- **M1**：`lockedField` 改成只 `DimFace` 不 `lockedControl`（丢掉那一淡）→
  `TestTheLockedFieldFadesOnce` 的 face / counter-side 两读变红（锁定后不再是半程）。
- **M2**：`lockedField` 改成 `LitFace(lockedControl(e))`（灯又亮回去）→
  新增 pin 的①与 `TestTheLockedFieldFadesOnce` 的冠线臂变红。
- **M3**：`DimFace` 换成空函数（只留 `lockedControl`）→ 同上，确认熄灯确实是这条 pin 的对象。
