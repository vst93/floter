# P11-am 规格 · 「这块面没有 accent」的判据要跟着 accent 走

## 意图（一句话）

上一轮（P11-al）把 accent 从红换回参考的蓝，**两条「这面没有 accent」的 pin 从此永远不会红**：
它们的判据是字面红的单方向差（`R−G > 24 && R−B > 24`），蓝 accent 满足不了；本轮把判据换成
「与 accent 自身的色度方向（chroma）比」，并把这条历史债冻成一条自己的 pin。

## 为什么现在做（证据）

选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）→ 合约未落地条目（`§6` 全表
`已落地`，只剩 `⌘K` 动作面＝拍板）→ **上一轮报告「仍待」第 1 条**（P11-al 报告 §7 明写
「这是本轮明确留下的债，不是已收口」）→ 界面质感。四条拍板项（caret、3.04:1、滚动缓冲、
`⌘K`）都卡在用户决定上，所以本轮领这条债。

| 证据 | 出处（行号本轮实测） |
|---|---|
| accent 现在是蓝 | `internal/theme/theme.go:661`（浅 `#064AA6`）、`:749`（深 `#8FB7FF`） |
| 判据钉的是红 | `internal/launcher/outputaccent_test.go:119-125`：`ri-gi > 24 && ri-bi > 24 && absInt(gi-bi) <= 8` |
| 第二条同款内联判据 | `internal/launcher/raycast_test.go:347`（`launcherListCarriesNoAccent` 的整帧扫描） |
| 蓝 accent 的色度 | 浅 `R−B = −160`、`G−B = −92`；深 `R−B = −112`、`G−B = −72` —— 三条红判据全部为假 |
| 后果 | `:85` 的 `carriesAccent(...)` 恒假 ⇒ `accent > 0` 永假 ⇒ `TestTheOutputSelectedRowIsTheNeutralChosenRung` 的反面半边、`TestTheLauncherListCarriesNoAccent` 整条，**都失去了失败能力** |

同一族的另一半（`internal/terminalui`）本轮复核**无需改**：那几处用的是
`B−R`（P11-al 已翻）或「通道散度」（`accentguard_test.go:69-75`、`drawermetrics_test.go:228`、
`redframe_test.go:88`），与色相无关，蓝 accent 一样能抓。P11-al 报告把它和 launcher 并提是笼统说法。

## 落地项（位置 → 现状 → 应改成）

1. `internal/launcher/outputaccent_test.go:115-125` → 现状：`carriesAccent(c)` 只认红。
   应改成：`accentRuler`（把 accent token 的色度向量 `(R−B, G−B)` 当一条射线，像素的色度
   在射线上的投影 `share` ≥ 0.05 且离射线的残差 ≤ `4 + 0.2·|chroma|` 才算「带 accent」）。
   **理由**：accent 与中性底混合（淡染、keyline、抗锯齿字形）都只是在这条射线上缩放 alpha，
   与色相无关；换色时判据自动跟上，不需要再翻一次。
2. `internal/launcher/raycast_test.go:344-348` → 现状：同一条红规则内联手写一遍。
   应改成：调同一把 ruler（`newAccentRuler(tokens.Theme.Accent)`），删掉内联条件与那段
   解释「accent 是红」的注释。
3. 新增 `internal/launcher/accentruler_test.go` · `TestTheAccentRulerFollowsTheTokenNotTheRed`
   —— 把这笔债冻住：用**当前 token** 合成像素（纯 accent、`AccentTintQuiet` 0.065 覆盖在
   面板上、`AccentEdge` 0.36 覆盖在面板上）必须被判「带 accent」；中性灰、`Warning` 墨、
   系统图标的琥珀面、一个外来绿必须**不**被判。同一批像素上，**退役的红规则必须判 0**
   （正是它失去失败能力的那件事），ruler 的 `norm == 0`（accent 变中性）时 `t.Fatal`。
4. 新增 `internal/theme/theme_test.go` · `TestTheAccentFamilyAlreadyClearsTheReadableContrast`
   —— 收 P11-al「仍待」第 2 条：`palette.contrast()` 不碰 accent，本轮用同一把
   `contrastRatio` 把它量清楚并钉住：accent 当墨（面板 / 选中行）与 `AccentText` 在 accent 填充上
   两套外观、两档对比度都 ≥ 4.5:1；accent 填充对面板（非文字控件）≥ 3:1；且
   **高对比度档不得改动 accent 一族**（`high.Accent == plain.Accent` 等）。

## 不改什么（行为契约零改动红线）

产品代码**一个字节不改**（`git diff -- internal/ ':!*_test.go'` 必须为空）。高度链、键盘模型、
会话管理、设置写回、i18n 文案、命令接线、全部墨与几何、全部既有 V pin 原样。

## 门槛与 PASS 基线（动工前实测）

`gofmt -l cmd internal` 空 · `go vet ./...` 三平台净 · `go test -count=1 ./...` 全绿 ·
PASS **2365** · SKIP 27 · 函数 1934 · 三平台 build 过。

## mutation 计划（≥2 条）

- **M1** 把 `outputRow` 的选中行填充改回 `AccentTintQuiet`（旧语言）→
  `TestTheOutputSelectedRowIsTheNeutralChosenRung` 必须红（新 ruler 看得见），而**旧红规则在同一帧上
  计数为 0**——这两件事一起构成「修前红 / 修后绿」的证据。
- **M2** 把 `accentRuler.carries` 退回单方向红判据 → `TestTheAccentRulerFollowsTheTokenNotTheRed` 必须红。
- **M3** 把深色 `accent` 调成 `#EFEFEF`（近中性）→ 主题对比 pin 必须红。
