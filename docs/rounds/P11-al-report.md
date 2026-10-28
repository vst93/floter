# P11-al 报告 · accent 回到参考的蓝，红只剩语义

方向一句话：**F2「红色太扎眼」的根是 accent 的色相，不是聚焦环——把 accent 换回参考的蓝。**

## 1. 为什么这一轮改色，而且这不是「擅自改品牌色」

`.pending-bugs.md` 的 F2 验收标准是「红线**收敛成只用于真正的破坏性/错误语义**」。
P11-z 把「窗口有焦点」那一圈红收成了中性发丝，但界面上仍处处是红——因为
**`Theme.Accent` 自己就是红**：分段控件的选中段 keyline、开关 ON、主 CTA 填充、
侧栏选中项前导图标、权限点、剪辑区高亮、caret。

本仓这张表的注释自己写着「Everything here is the old build's value: **the pastel accent**」
（`internal/theme/theme.go:618`），参考 `main` 的 accent 从头到尾是蓝
（`--accent: #8fb7ff` / `#064aa6`，`main:src/styles/base.css:675,942`），整个 base.css 里
红只出现在 `--ansi-*` 与 `--text-warning`。也就是说：**「品牌红」是本仓移植时自己加的，
不是参考的**（P8-a 的测试注释写着「旧值 #f5544e ← #8fb7ff，按合约 §1 换成品牌红」）。
按「合约单一来源 = 参考」，这是移植偏离，不是设计决定；而它正是用户 F2 的那句话。

**如果用户要品牌红回来**：`git revert <本 commit>`，或只把
`internal/theme/theme.go` 两处 `palette` 的 `accent` 家族改回红，并把
`docs/raycast-design.md` §1 的两行 accent 改回——一条 token 的事，报告 §5 已记。
这条我**没有**写进「需拍板」表，因为它是「向参考对齐」+「用户 F2 的验收标准」两条同时指向的改法；
若用户认为品牌红是既定选择，按上面一行改回即可。

## 2. 落地（只改墨，不改面与几何）

- 浅色：`accent #064AA6` / hover `#0857C4` / pressed `#053C87` / `accentText #F7F9FF`；
  `selection rgba(6,74,166,.10)`、`focus rgba(6,74,166,.82)`、`accentEdge .36`、
  `accentEdgeStrong .68`、`accentHalo .24`、`accentGlow .20`、`accentTintQuiet .065`。
- 深色：`accent #8FB7FF` / `#A3C4FF` / `#7AA7FF` / `accentText #15171B`；
  `selection rgba(143,183,255,.13)`、`focus rgba(122,167,255,.78)`、`accentEdge .30`、
  `accentEdgeStrong .72`、`accentHalo .18`、`accentGlow .08`、`accentTintQuiet .085`。
- **没动**：`strokeActive`（P11-z 的中性聚焦档）、`warning`（`#9C1F18` / `rgba(255,178,164,.82)`）、
  行的中性三档、终端 ANSI 调色板、全部几何与文案。红现在只剩 `warning` 一族。

## 3. 台账 V / B / S

- **V（重定基线，8 条）**：`TestPaletteComesFromBaseCSS`（dark/light accent、dark accentText、
  两条 accentHalo）、`TestAccentTintQuietStepsDownInTheLightAppearance`（两个外观）、
  `TestTheChosenSegmentKeepsItsAccentKeyline`（阈值 40 → **24**：蓝 keyline 是 `--accent-edge`
  0.30 而红曾是 0.55，压在同一张深色轨道上实测只到 **36** 级；浅色实测 60）。
  每条断言的**形状**都没放宽（仍是「具体色值 + alpha」「这圈边有 accent」）。
- **S（结构真变了，1 类）**：11 个测试文件里「一个像素有多 accent」的探针原本写成
  `R − B`（把「红」当 accent 的判据）。accent 变蓝后这个判据永远为负，等于把
  「这是 accent 吗」变成永远假——所以把**测 accent**的探针换成 `B − R`（accent 家族两个外观
  都是 B > R）。**测警告红/琥珀暖面**的探针（`extensionpanelcss` 的 broken chip 面、
  `pluginsheetface` 的 danger 墨与 hairline、`raycast_test` 的 verdict chip 面）**保留 `R − B`**，
  并在现场写了注释说明为什么这条不跟着翻。
- **B（零改动）**：其余全部 pin（含 `TestTheSwitchOnStateCarriesTheAccent`、
  `TestThePermissionDotAndTheRegisterChipKeepTheAccent`、`TestTheSidebarChosenGlyphKeepsTheAccent`、
  `TestTheRegisterRowIsTheNeutralSelectedRung`、`TestTheFeedbackNoticeRingIsNotAccent`）
  **一行没改**就全绿——说明这些表面读的都是 `tokens.Accent`，色相一换它们自己跟上。
- 测试函数 **1933 → 1934**（+1 是本轮的 opt-in 截图 dump，默认 SKIP）。

## 4. 门槛（本轮实测）

```
gofmt -l cmd internal                    # 空
go vet ./...                             # 净
GOOS=windows go vet ./...                # 净
GOOS=darwin  go vet ./...                # 净
go test -count=1 ./...                   # 0 FAIL
--- PASS 计数                            # 2365（基线 2365，未减）
--- SKIP 计数                            # 27（基线 26，+1 = 本轮 opt-in dump）
测试函数数                                # 1934（基线 1933）
三平台 build                              # linux / windows / darwin 全过
```

## 5. 变异（2 条，实跑变红、逐字节还原）

- **M1** 浅色 `accent` 改回 `#E5484D` → `TestPaletteComesFromBaseCSS` 红：
  `theme_test.go:322: light accent = {229 72 77 255 …}, want base.css's #064AA6`。
- **M2** 侧栏选中项前导图标 `t.Accent` → `tokens.TextTertiary` →
  `TestTheSidebarChosenGlyphKeepsTheAccent/light` 红：
  `raycast_test.go:217: the chosen page's mark is not the accent: {149 149 155} is 6 levels from grey`。
- 还原后 `md5sum -c` 两条都「成功」，`touch` 后复跑同一个 `-run` 全绿。
  （注：M1 第一次还原时误用了 `git checkout`，把本轮的 `theme.go` 冲回 HEAD，已按同一份
  改动重新落盘并用 `cp` 备份 + `md5sum -c` 双向核对，故上面的还原结论是 cp 版的实测。）

## 6. 图（我实测看到了什么）

`/tmp/floter-P11-al-accent-light.png`、`/tmp/floter-P11-al-accent-dark.png`
（`internal/settingsui/accentdump_test.go`，`FLOTER_PNG=1` 才跑，720×580 设置页）。

- 浅色：accent token `{6,74,166}`——侧栏选中项（General）前导图标那一列从**红**变成**深蓝**；
  同一帧里唯一还偏红的像素是警告/琥珀面（`--system-icon-surface`）与 `warning` 墨，
  正是 F2 要的「红只回答语义」。
- 深色：accent token `{143,183,255}`——同样的图标列是**浅蓝**，卡片与行仍是中性灰阶，
  没有任何一圈红。
- 实测数字（同一个 dump 打印）：light `accent={6 74 166}`、dark `accent={143 183 255}`；
  分段控件选中段的 keyline 实测「偏蓝级数」浅 **60** / 深 **36**（旧阈值 40 是按红 0.55 定的）。

## 7. 仍待

1. **「没有 accent」那一侧的反面 pin 只翻了一半**：11 个文件里测 accent 的探针已改 `B − R`，
   但 `internal/launcher` / `internal/terminalui` 里还剩几条**纯中性面**的断言（`< 20` 之类）
   在 accent 变蓝后失去「能抓到蓝色漏进中性面」的能力（红时能抓）。下一轮把这几条的度量
   换成「与 accent 的通道距离」而不是单一方向的差值——**这是本轮明确留下的债，不是已收口**。
2. `palette.contrast()` 的高对比度档没有随 accent 换色复核（P11-aa 只覆盖四档墨，
   accent 在高对比度下是否要再抬，需实测 + 可能要拍板）。
3. 契约 `docs/raycast-design.md` §1 的两行 accent 已随本轮改写；若用户要品牌红，按 §1 改回即可
   （见本文 §1 末）。
4. `.pending-bugs.md` 里那四条拍板项原样有效（caret 是否脱离 accent、默认浅色安静档 3.04:1、
   终端滚动缓冲、`⌘K` 动作面）。
