# P13-fd 规格：空态那句话的居中带，比同一块面板的列表少算了一条底沿

轮号 P13-fd（HEAD 前 P13-fc）。方向：通用队列 · **空态满态**（`liquid-glass.md` §6.124 的
`124-5`：启动器无结果态的密度）。表面 = **启动器卡片 · 安静页（无结果）**。
合约新增 **§6.125**。

## 意图（一句话）

空态那句话此前按「上 `listEdge` / 下 `Space(3)`」居中，而同一块面板里列表的下沿是
`Space(0.5) + statusBand` —— **同一个底沿，两页两个数**：既少了 `1u` 的 tail（3u vs 2u），
也没把停靠带（`statusBand`）还回去。于是按住 ⌘ 让带子出现时，**列表一行不动，那句话却往下跳
13.5px**。

## 证据（先量，真窗口 720×charged，en/zh × 深/浅四臂逐值相同）

| 臂 | charged | 那句话的盒 | 上方空气（字段下沿 50.4 → 盒顶） | 下方空气（盒底 → 卡底） |
|---|---|---|---|---|
| 打字无结果（`zzzz`） | 113 | `Y=69.1 H=18` | **18.7** | **25.9** |
| 同上 + 按住 ⌘（停靠带在场） | 140 | `Y=82.6 H=18` | **32.2** | **39.4**（带顶 113） |
| 剪贴板空态（chips 在场） | 131 | `Y=87.1 H=18` | **15.1**（chips 带下沿 72） | **25.9** |

- 第一行与第三行：下方空气恒为 **25.9**，上方 18.7 / 15.1 —— 那句话**压在它回答的东西上**，
  离卡底反倒远 7~10px。
- 第二行是本轮的**病**：带子出现时 charged 113 → 140，句子跟着下移 **13.5px**（69.1 → 82.6）。
  同一帧里列表的行为是**不动**（`launcher.go` 列表的 `Padding(…, Space(0.5)+a.statusBand(c), …)`
  把带子从内容里扣掉）——两页对同一个带子两个答案。
- 数字来源：`Padding` 四值形式是 (top, right, bottom, left)（mygo `ui/element.go:720` `edges`），
  所以空态列的下内边距是 `Space(3)`=10.8，而列表是 `Space(0.5)`=1.8 + 带高 27。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|---|---|---|
| fd-1 | `internal/launcher/launcher.go` 安静页那一列（`} else if quietPage {` 分支） | `Padding(listEdge, t.Space(3), t.Space(3), t.Space(3))` | 下内边距 `t.Space(0.5)+a.statusBand(c)` —— 与列表**同一个表达式**（面板自己的 tail + 停靠带） |
| fd-2 | 同上，左右内边距 | `Space(3)` | 不动（`SingleLine()` 的句子用不到，但它是这条列自己的呼吸） |
| fd-3 | 墨 / 字阶 / 居中 / `quietStatement` | `Emphasis` + `TextStrong`、`.Center()` | **一字节不动**（§4.7 / §6.47） |
| fd-4 | 高度链 | `Geometry.Height` / `RowCount` | **一字节不动**：本轮只挪画出来的那句话，不挪窗口 |

新数（预期）：打字无结果 69.1 → **73.6**（+4.5）；按住 ⌘ 82.6 → **73.6**（−9.0，带子出现
**不再移动**）；剪贴板空态 87.1 → **91.6**（+4.5）。上方/下方空气：23.2 / 21.4（无带）、
23.2 / 21.4（有带，量到带顶 113）、19.6 / 21.4（chips 臂）—— 三臂都从「高 7~10px」收到
**1.8px 以内**。

## 行为契约零改动红线

高度链（`Geometry.Height`、`RowCount`、`WindowHeight`）、键盘模型（`HoldModifiers` 的
选中/滚动路径）、会话管理、设置写回、i18n 文案（`copy.NoResults` / `ClipboardEmpty` 不动）、
命令接线：**全部零改动**。视觉轮只改这一列的下内边距。

## 门槛命令与 PASS 基线（开工实测）

```
gofmt -l cmd internal                # 必须空
go vet ./... ; GOOS=windows go vet ./... ; GOOS=darwin go vet ./...
go test -count=1 ./...               # 基线 3394 PASS / 0 FAIL / 186 SKIP（HEAD f6b0827f）
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux/windows/darwin go build ./...
```

## 台账预判

- **V 1**：`TestTheEmptyPageSaysSoUnderTheField` 把「带底 = 卡底 − `3*4*designUnit()`」写死在
  断言里，必须重定基线成 `Space(0.5)`（并在注释里写 `P13-fd 重定基线`）。
- **B 0**：其余（高度链、字段带、chips 带、键盘）一条都不许红。
- **S 0**。

## mutation 计划（≥2 条，每条说明会让哪个 pin 变红）

1. **M1**：下内边距退回 `t.Space(3)`（不带 `statusBand`）→ 新 pin 1（三臂的居中）+ 新 pin 2
   （带子出现时不移动）双双变红。
2. **M2**：只加 `statusBand`、把 tail 仍留 `Space(3)`（即 `Space(3)+statusBand`）→ 新 pin 1
   红（无带的两臂仍是 69.1/87.1），pin 2 绿 —— 证明「tail」与「带子」是两笔账。
3. **M3**：上内边距 `listEdge` → `fieldRow`（等价写法之外的另一刀）→ 若绿，记成已知覆盖边界
   （本轮不动上沿）。
