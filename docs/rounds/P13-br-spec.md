# P13-br · 设置页分组间距：8u 与 10u 的 2 个像素，先出图再决定

> 队列来源：`docs/liquid-glass.md` §6.35 的 **35-3**，也是全表**唯一**的 `未开始` 行
> （P13-bq 留给本轮的下一格）。目标轴：**细节打磨 · 空间感**。

## 一、意图

`sectionGroup` 的组内节奏是 8u（`rows.go:81`，`.settings-section { gap: 8u }`），
材质组 `sectionMaterial` 的组内节奏是 10u **加上** 上下各 10u 的 padding（`rows.go:114`，
`.settings-section--material { gap: 10u; padding: 10u 0 }`）。同一个角色（「组头 → 卡片」之间
的空气）在同一页上有两个刻度，差 **2 个物理像素**（`Space(0.5)` = 0.5 × Spacing ≈ 1.8px，
UI 步长 `small` 下四舍五入到 2px）。

P13-bq 的处置是**先量，不许先改**，并写死「下一格：先出图看这两个角色读不读得出来
（General 页两张相邻分组卡的间距，深/浅两套），**再决定改不改**」。这一轮就是那条「再决定」。

**为什么让它更有空间感**：如果 2px 读不出，那它就是**一个没有收益的第二刻度**——
删掉它，页面上的间距角色从「8 / 10 混着」回到「8 是组内唯一的一档」，而材质组真正的身份
（10u 的 padding，比别组多出 36px 的呼吸）反而更清楚。如果 2px 读得出，则保留并把
「材质组比别组松」写成一条可量的账。

## 二、动手前必须先拿到的读数（不许先改）

| # | 要量什么 | 怎么量 |
|---|---|---|
| M-1 | 每个分组**盒**的 top/bottom | `ui.Tester.Find(sectionLabel(title))`——`sectionGroup` 把组名挂成组的 `Label`，这个盒就是组自己的盒（`rows.go:97`） |
| M-2 | 相邻两组盒之间的实际像素 | 上一组 `Y+H` → 下一组 `Y`；`pageColumn` 是 `Space(5.5)` = 22u，材质组的 padding 在**组盒内部**，所以两组的盒间距应当**五组同值** |
| M-3 | 组内「组头 → 卡片」的实际像素 | 组盒 top → 组头文字 top；组头文字 bottom → 第一行文字 top（卡片有 9u 行 padding，五组同构，差值只来自组 gap） |
| M-4 | 深浅两套是否同值 | 同一次测量跑 `light` / `dark` 两遍，逐项比对 |

## 三、落地项（量完之后按读数二选一，写死在报告里）

- **分支 A（2px 读不出）**：`sectionMaterial` 的组 gap 从 `Space(2.5)` 改回共享的
  `Space(2)`，**保留** `padding = Space(2.5)`；`sectionGroupSpaced` 的签名因此只剩
  「padding 可以不同」一个自由度。合约里新增 §6.36 一行 `已落地`，写明旧值 → 新值。
- **分支 B（2px 读得出）**：产品代码零改动，§6.35 的 35-3 改判 `故意不做（量过：…）`，
  并把读数写进合约。
- 两条分支都**必须**新增一条 pin，把「组内节奏只有一档」或「材质组的 10u 是它自己的
  身份、且组盒间距五组同值」钉住，否则下一轮又会漂。

## 四、行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**一个字节都不动**。
本轮只可能动「组内的一个 gap 值」与「一条断言」。

## 五、门槛与 PASS 基线（实测，不抄旧报告）

```
gofmt -l cmd internal                                 → 空
go vet ./...                                          → 净
GOOS=windows go vet ./...  /  GOOS=darwin go vet ./... → 净
go test -count=1 ./...                                → 全绿
go test -count=1 -v ./... | grep -c -- "--- PASS"      → 基线 3207（只许涨）
```

## 六、变异计划（≥2 条，实跑变红并逐字节还原）

| # | 注入 | 期望变红的 pin |
|---|---|---|
| M1 | `sectionMaterial` 的 gap 改回 `Space(3)`（故意造第二个刻度） | 新增的组内节奏 pin |
| M2 | `sectionGroupSpaced` 的 `Padding(padding, 0)` 改成 `Padding(0, 0)`（材质组丢掉自己的呼吸） | 新增的材质组身份 pin |
| M3 | `sectionGroup` 的 gap 改成 `Space(2.5)`（普通组被材质组同化） | 新增的组内节奏 pin 的另一半 |

## 七、出图

`ui.Tester` → `/tmp/floter-P13-br-general-{dark,light}.png`，General 页、深/浅两套，
opt-in（`FLOTER_PNG=1`），与 P8-c / P11-al 的 dump 同一约定。
