# P13-di 规格 · 快捷键页的半成品行：它缺的是一格键，不是整行的亮度

方向一句话：**设置页 → 快捷键页那条「正在做」的自定义快捷键行，不再用整行 62% 的乘法假装
「还没写完」**——它和一条已保存的行是同一行、同一套墨，缺的那一格键由空的录制槽说。

## 1. 意图与证据

任务书「下一轮 = 细节打磨；下一格从『状态反馈』的另一半（禁用 / 只读面）或空态满态里挑，
先量再改」。§6 差距表已无 `未开始` 行；禁用面一族的成员（`disabledActionButton` /
`busyActionButton` / `extensionIconButton` / `lockedControl` / `disabledSwitch` / 卸载整卡 /
`commandPlate`）已按 §6.2b-6 与 §6.15 落地过。剩下的**唯一没量过**的一类，正是**不是
`Disabled`、只是「还没写完」**的那一行：

- `internal/settingsui/custom.go:258` `customDraftRow` —— 参考的 `.settings-row--muted`
  （`settings.css:564-569`）整行 `opacity: 0.62`，移植照做：
  改前 `custom.go:256` `face.Muted = !complete`。
- `internal/settingsui/rows.go:1372-1374` `rowFace.Muted` 的文档说得很清楚：
  「Muted dims the whole **row** to the reference's 0.62」。
- 消费点改前在 `rows.go:1437`：`if face.Muted { row = row.Opacity(0.62) }` ——
  框架的 `Opacity` 是把整棵子树**合成到它身后的面**上，是一次**乘法**。

这一行有两档墨（标题 `text`、提示 `textMuted`），而乘法只有一个因子。先量（`ui.Tester`，
整行盒扫描，`litSum` 三通道和，1 亮度级 = 3；本文件的表与 `draftrowink_test.go` 头注释同源）：

| 外观 | 因子 | 行标题墨 | 行内提示墨（推导 `0.62·ink + 0.38·面`） |
|---|---|---|---|
| 浅 | 1.00 | **83**（27.7 = `text` 27） | 128 = `textMuted` |
| 浅 | 0.62 | **322**（107.3 ≈ `textSecondary` 107） | **173**（低过 `textTertiary` 142） |
| 浅 | 0.75 | **238**（79.3，两档之间） | 152（两档之间） |
| 浅 | 0.50 | **398**（132.7，`textMuted` 128 与 `textTertiary` 142 之间） | 179 |
| 深 | 1.00 | **713**（237.7 = `text` 237） | 133 = `textMuted` |
| 深 | 0.62 | **476**（158.7，`textSecondary` 154 + 5） | **99**（低过 `textTertiary` 110） |
| 深 | 0.75 | **560**（186.7，两档之间） | 111 |
| 深 | 0.50 | **398**（132.7 ≈ `textMuted` 133，另一对档之间） | 88 |

两条结论，都是这一轮的判据：

1. **一个乘法不可能同时是两个档。** 0.62 让标题恰好冒充 `textSecondary`（浅色一个字节都不
   差），同时把这一行最安静的那一档墨推到阶梯之外；换 0.75 / 0.50 只是把「落在两档之间」
   从提示挪到标题。阶梯上没有一个因子能同时命名这两档。
2. **它洗掉的正是唯一能让这一行写完的控件。** 选择器的受光边、命令输入框的面、录制键的
   键帽、`Bind` 键的面，全被合成为 62% 亮的面——§3 说受光边是「这块面可以被碰」的记号，
   而这是整页里唯一**必须**被碰的那一块。这不是「淡了一点」，是「不该淡的淡了」。

## 2. 逐条落地项

1. `internal/settingsui/rows.go` `rowFace`：删掉 `Muted bool` 字段，把它改写成一条**为什么
   这一格故意不存在**的注释（附上上面那张表的四个因子）。→ 现状：字段 + 消费点；
   应改成：注释里留证，代码里没有。
2. `internal/settingsui/rows.go` `cardRow`：删掉 `if face.Muted { row = row.Opacity(0.62) }`。
   → 行不再被合成到它的地面上；行里的每一块面、每一条受光边、每一档墨都留在原处。
3. `internal/settingsui/custom.go` `customDraftRow`：`face.Muted = !complete` 改成零值
   `rowFace{}`，并把函数注释改成 P13-di 的说法。**几何、控件、键盘、写回一个字节不动。**

「还没写完」这件事此后由**唯一缺的那一格**说：录制槽是空的（未武装时是一格空键位），
武装后是 `Press keys…`。`TestCustomShortcutsPage` 钉的正是那个人能看到的事实。

## 3. 行为契约零改动红线

- 高度链：`cardRow` 的地板、分隔线、1px 行距、`rowInset` 全部不动（`MinHeight(floor)` 那几行
  一个字节没碰）；洗与不洗不参与布局（`Opacity` 是画笔的合成，不是盒子）。
- 键盘模型：不加 `.Disabled`、不改焦点顺序、不改 `customRecording` / `customRecordRow` 的语义。
- 会话管理 / 设置写回 / i18n 文案 / 命令接线：零改动（本轮只删一条画笔合成与一个字段）。

## 4. 门槛命令与 PASS 基线（实测，改前）

```
gofmt -l cmd internal              → 空
go vet ./...                       → 净
GOOS=windows go vet ./...          → 净
GOOS=darwin  go vet ./...          → 净
go test -count=1 ./...             → 全绿
--- PASS 计数                      → 3311（基线）
--- FAIL / SKIP                    → 0 / 148
测试函数数                          → 2467
三平台 build                       → 过
```

改后预期：PASS **3312**（+1 条本轮 pin），SKIP **149**（+1 opt-in 截图 dump），
测试函数 **2469**（+2）。

## 5. mutation 计划（≥2 条，逐条说明会让哪条 pin 变红）

- **M1** 把 `0.62` 那道洗原样搬回来（`rows.go` 的 `if face.Muted` + `custom.go` 的
  `face.Muted = !a.customRowComplete()`）→ `TestTheUnfinishedShortcutRowWearsTheSameInkAsASavedOne`
  的两条断言在深浅各红一次（墨 `322` 对 `83` / `476` 对 `713`，对比 `420` 对 `659` / `432` 对 `669`）。
- **M2** 保留 M1 的机制、把因子换成 `0.75` → 同一条 pin 红，数字换成 `238/504` 与 `560/516`
  ——证明这条 pin 钉的是「乘法」本身，不是 `0.62` 这个数。
- （旁证，已在测量阶段跑过）因子 `0.50` → `398/344` 与 `398/354`，同样红。
