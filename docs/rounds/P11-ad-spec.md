# P11-ad 规格：同一个窗口里的两种列表只有一条文字列

## §0 意图一句话

启动器列表与它自己的输出面板是**同一张卡里的两种列表**，行标题却落在两个列上（54u 与
52u）——本轮让输出面板的行几何就是启动器列表的行几何，于是「一条文字列」在两种列表里
是同一个数，而不是各算一份。

## §1 为什么是这一条

- 用户实测 F1/F2/F3（终端异常字符、红线扎眼、类玻璃边）已由 P11-y / P11-z / P11-aa
  收完；合约 §6 全表 `已落地`，唯一未落地的 `⌘K` 动作面要动键盘模型 → 拍板项。
- 上一轮（P11-ac）把两个表面的**分组头**钉到各自的标题列上，并在「仍待」第 4 条留下
  本轮的缺口：输出面板的行 gap 是 8u、启动器列表是 10u，于是两个表面的标题列差 2u
  （52 vs 54）。P11-ac 只统一了「分组头 == 本表面标题列」，没有统一「两种列表 == 一个
  文字列」。
- 证据（`ui.Tester.Find()` 的矩形，HEAD `be3357d` 实测，本轮规格写下时量的）：

| 表面 | 元素 | 现状 x | 行 gap | 应改成 |
|---|---|---|---|---|
| 启动器列表 | 行标题 `Editor` | 54.0 | 10u | 54.0（不动） |
| 启动器列表 | 分组头 `Recently launched` | 54.0 | — | 54.0（不动） |
| 输出面板 | 行标题 `Alpha` | **52.0** | 8u | **54.0** |
| 输出面板 | 分组头 `Group A` | 52.0 | — | 54.0 |

两个数都是「面板/卡片 8u + 行内 padding 8u + 图标列 28u + 行自己的 gap」的和，唯一
不同的那一项就是 gap（`internal/launcher/output.go` 的 `Gap(t.Space(2))` vs
`internal/launcher/launcher.go` 的 `Gap(t.Space(2.5))`）。P11-ac 的注释把这个和抄成了
字面数（`Margin(0,0,0,t.Space(9))` = 36u），所以 gap 一动它必须跟着动。

## §2 逐条落地项

1. `internal/launcher/output.go:546`（`outputRow` 的行盒）
   现状 `Gap(t.Space(2))`（8u）→ 改成 `Gap(t.Space(2.5))`（10u），与启动器列表的行
   同一条内部节奏（图标列与文字列之间、文字列与行尾控件之间都是这一份 gap）。
2. `internal/launcher/output.go:536`（分组头的 `Margin-left`）
   现状 `t.Space(9)`（36u）→ `t.Space(9.5)`（38u），因为和里那一项 gap 从 8u 变成 10u；
   分组头仍然落在**本表面标题列**上（52 → 54）。
3. 注释按事实改：把「= 36u 的 Margin-left，加上列表自己的 8u 即 52u」改成 38u / 54u，
   并写清「这一份 gap 就是启动器列表的 10u，两种列表只有一条文字列」。

不改：字阶、weight、墨、band（6u/4u）、行高、图标列 28u、keycap 槽、选中/悬停档、
滚动与键盘模型。**纯墨与几何。**

## §3 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线一律不动。本轮只动两处横向
间距（8u → 10u）与一处 `Margin-left`（36u → 38u）。

## §4 门槛命令 + PASS 基线（本轮实测的当前值）

```
gofmt -l cmd internal                # 必须空
go vet ./...                          # 必须净
GOOS=windows go vet ./...             # 必须净
GOOS=darwin  go vet ./...             # 必须净
go test -count=1 ./...                # 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 只许涨
```

基线（HEAD `be3357d` 实测）：**2351 PASS / 0 FAIL / 20 SKIP / 1913 测试函数**。
本轮新增 1 条 pin + 1 条 opt-in dump → 期望 **2352 PASS / 20 SKIP / 1915 函数**。

## §5 mutation 计划（≥2 条，每条说明会让哪个 pin 变红）

1. 把 `output.go` 的行 gap 改回 `t.Space(2)`（8u），`Margin-left` 保持 38u：
   `TestTheTwoListsShareOneTextColumn` 红（输出面板标题 52 vs 启动器 54），
   `TestTheGroupHeadingSitsOnTheRowsTextColumn` 也红（分组头 54 vs 标题 52）。
2. 只把分组头的 `Margin-left` 改回 `t.Space(9)`（36u）：`TestTheGroupHeadingSitsOnTheRowsTextColumn`
   红，新 pin 仍绿——两条 pin 分别守着「头对行」与「面对表面」。
3. 把行 gap 改成 `t.Space(3)`（12u）而不动分组头：新 pin 红（56 vs 54）。
