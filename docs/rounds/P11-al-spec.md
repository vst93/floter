# P11-al 规格 · accent 回到参考的蓝，红只剩语义

## 意图（一句话）

用户实测 F2「整体红色线条太扎眼」的**根因不是**聚焦环（P11-z 已收），而是 `Theme.Accent`
本身：本仓把 accent 定成「品牌红」`#F5544E`/`#E5484D`，而**参考 `main` 的 accent 一直是蓝**
（`--accent: #8fb7ff` / `#064aa6`）。把 accent 换回参考的蓝，红在本仓就只剩
`warning`（警告/破坏性）一个语义。

## 为什么现在做（证据）

| 证据 | 出处 |
|---|---|
| 参考深色 accent 是蓝 | `main:src/styles/base.css:675` `--accent: #8fb7ff`、`:676` `--accent-contrast: #15171b` |
| 参考浅色 accent 是蓝 | 同文件 `:942` `--accent: #064aa6`、`:943` `--accent-contrast: #f7f9ff`；注释写明「the same pastel blue … barely there on white」 |
| accent 家族其余档 | `:677-695`（深）/ `:945-956`（浅）：`--accent-tint/.085/.19`、`--accent-edge .30/.36`、`--accent-edge-strong .72/.68`、`--accent-glow .08/.20`、`--accent-ring`、`--accent-halo .18/.24` |
| `::selection` 就是 accent-tint | `main:src/styles/base.css:1300` `::selection{background:var(--accent-tint)}` |
| 本仓这张表自己说要抄参考 | `internal/theme/theme.go:618`「Everything here is the old build's value: **the pastel accent**, the translucent … ladder」——注释写 pastel，值却是红 |
| 参考里根本没有红品牌色 | `git grep -nE "#[0-9a-f]{6}" main -- src/styles/base.css`：红只出现在 `--ansi-*` 与 `--text-warning` |

用户原话的验收标准（`.pending-bugs.md` F2）：「收敛成『只用于真正的破坏性/错误语义』」——
accent 换蓝之后，红在界面里只剩 `t.Warning` 这一族。

## 落地项（位置 → 现状 → 应改成）

1. `internal/theme/theme.go` 浅色 `palette`：`accent #E5484D / hover #F0555A / pressed #CC3B40 /
   accentText #FFFFFF` → `#064AA6 / #0857C4 / #053C87 / #F7F9FF`；`selection rgba(229,72,77,.16)
   → rgba(6,74,166,.10)`；`focus rgba(229,72,77,.82) → rgba(6,74,166,.82)`；
   `accentEdge .55→.36`、`accentEdgeStrong .85→.68`、`accentHalo .20→.24`、`accentGlow .12→.20`、
   `accentTintQuiet .10→.065`（RGB 全换）。
2. 深色 `palette`：`accent #F5544E / #FF6B62 / #DC4640 / #1B1B1D` → `#8FB7FF / #A3C4FF /
   #7AA7FF / #15171B`；`selection → rgba(143,183,255,.13)`、`focus → rgba(122,167,255,.78)`、
   `accentEdge .55→.30`、`accentEdgeStrong .85→.72`、`accentHalo .20→.18`、`accentGlow .10→.08`、
   `accentTintQuiet .14→.085`。
3. **不动**：`strokeActive`（P11-z 已中性）、`warning`、`rowHover/rowSelected/rowPressed`、
   终端 ANSI 调色板、所有几何与文案。

## 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线**一个字节不改**：本轮只改墨。

## 门槛与 PASS 基线（动工前实测）

`gofmt -l cmd internal` 空 · `go vet ./...` 净（三平台）· `go test -count=1 ./...` 全绿 ·
PASS **2365** · SKIP 26 · 函数 1933 · 三平台 build 过。

## mutation 计划（≥2 条）

- M1 light `accent` 改回 `#E5484D` → `TestPaletteComesFromBaseCSS` 必红。
- M2 侧栏选中项前导图标改 `tokens.TextTertiary` → `TestTheSidebarChosenGlyphKeepsTheAccent` 必红。
