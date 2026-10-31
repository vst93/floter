# P13-bl 报告 · 门槛先修红，再把空态的边界钉住

## 这轮为什么让它更轻、更有空间感

一句话：**这一轮先把「看不见的坏」修掉，再证明一句话在最窄的窗口里仍然是完整的一句话。**
`HEAD` 上 `go test ./...` 是红的（差距表一行写坏了格），红着的门槛意味着后面所有
「视觉更轻」的判断都没有依据——先归零。第二件是空态那句唯一的陈述句：它没有省略号，
一旦比带子宽就会被硬切，两端不留记号；这轮量出它今天在 520u 上还留着 37u/37u 的余量，
把这个边界钉成 pin，**故意不改成换行**（换行会把居中的一句话变成两行、动到带子的视觉重心，
而今天没有这个需要）。

## 做了什么

1. **`docs/liquid-glass.md:1723`（§6.30 的 `30-1` 行）**：`\|` 转义竖线 →
   「此前是 `a.mode != nil` 或 `a.clipboard` 或 `a.browser`」。解析器按裸 `|` 切格，
   转义竖线让那一行的「状态」列读成 `a.clipboard \`。语义不变（仍 `已落地`）。
2. **`internal/launcher/quietclip_test.go`（新增 1 个测试）**：`TestTheQuietPagesSentenceIsNeverCut`。
   夹具是「浏览器插件关掉」那一页——安静页能画的最长一句（英文 74 字符）。量法：
   把带子按行切成「有墨的行段」，句子自己那一段的左右端点必须留在带子的 16u 内边距之内。

## 实测（`ui.Tester`，浅色，`frameHeight(maxResultSlots)`）

| 窗口宽 | 句子墨的横向范围 | 左余量 | 右余量 | 结论 |
|---|---|---|---|---|
| 520u | x = 49 … 471 | 37u | 37u | 完整，没被切 |
| 760u | x = 149 … 571 | 137u | 137u | 完整，没被切 |

带子内边距是 12u，pin 的门是 16u——今天两档都过，余量还很大。

## V / B / S 台账

- **V 0**（没有视觉数值变更：产品侧一行未改）
- **B 0**（`internal/ci` 的差距表契约是 B 类，红的是**文档措辞**，测试零改动）
- **S 0**
- 产品代码 `git diff -- internal/`：只有新增测试文件，零生产改动。

## 门槛（全部实测）

```
gofmt -l cmd internal   → 空
go vet ./...            → 净
GOOS=windows go vet ... → 净
GOOS=darwin  go vet ... → 净
go test -count=1 ./...  → 0 FAIL
PASS 3190（HEAD 3189 + 1）| SKIP 112 | 测试函数 2343 → 2344
GOOS=linux/windows/darwin go build ./... → 过
```

## mutation（2 条，实跑变红并逐字节还原）

- **M1**：`30-1` 行改回 `\|` 转义 →
  ```
  --- FAIL: TestTheLiquidGlassContractSupersedesTheRaycastOne
      liquidglass_contract_test.go:94: a gap-table row carries an unrecognised status
      "a.clipboard \\"; the four words are 未开始 / 已落地 / 需拍板 / 故意不做
  ```
  还原后 `md5sum -c` 成功（`a0b95383…`），`touch` 复跑绿。
- **M2**：空态句子的字阶 `tk.Emphasis` → `tk.Emphasis * 3` →
  ```
  --- FAIL: TestTheQuietPagesSentenceIsNeverCut
      quietclip_test.go:121: 520: the sentence's ink starts at x=15, inside the band's 16u
      margin — it is cut at the card's edge, not wrapped
  ```
  还原后 `md5sum -c` 成功（`98aeaee9…`），`touch` 复跑绿。

## 用 skill 的账（`qiaomu-design`）

- 开工读了 `SKILL.md` 的路由表与 `references/user-preferences.md`。
  本轮适用的两条：**P-1**（界面排版禁斜体——本轮零排版改动，无斜体新增）、
  **P-5**（可读性优先于气质——空态那句话是页面唯一的陈述，不允许被切）。
- `preflight.md`：**A**（反 AI 套路）本轮无新增视觉元素，过；**B**（设计读取一致性）
  空态仍走 §4.7 的陈述档（`Emphasis` + `TextStrong`），与 `settingsui.emptyState` 同一语法，过；
  **C**（动效抽查）本轮无动效改动，N/A；**D2**（中文页正文 ≥ 14px）**不过**——本轮量的是
  英文夹具，`tokens.Caption` 11px 的张力是沿用五轮的「需拍板」项，没有新证据。
- **如实记录**：本轮没有打开 `craft-loop.md` 的工艺密度菜单——这一轮没有产出视觉变更
  （产品侧零改动），硬凑一条「第 N 项」就是编造。下一轮做真视觉时补上。

## 仍待

1. **需拍板（沿用，第六份证据）**：`tokens.Caption` 11px 与 preflight D2「正文 ≥ 14px」的张力。
2. **需拍板（沿用）**：`ui/tooltip.go:133` 禁用元素不弹提示；`cardRow` 控制槽 `Shrink(0)`；
   多选箱宽度；`fieldSettingsButton` 按下档是否保留一对线（`litarm_contract_test.go:75`）；
   终端弧 S1b / S1c / S2 尾。
3. **本轮核过的三个「先量」候选，全部结案（都是「本来就对」）**：
   - `settingsui.go:5725` 的 `metadataCell`：标签 `Caption`+`TextTertiary`、值 `13px`+`TextSecondary`，
     上下两档都成立；
   - `settingsui/rows.go:971` 的 `sessionIconButton`：`resumable = Running && !Active`、
     `disabled = !Running`，与参考的 `disabled={exited || …}` 一致；
   - `shell/pinned.go:122` 的便条正文：`Body` + 1.45 + `TextSecondary`，与
     `shell/detach.go:572` 的 `detachedText` **逐字节同一组值**——两个窗口是同一个表面，本来就一致。
4. **下一格候选**：安静页的**六级 chips**（空页面上六个滤片 + 一条旁注仍在承诺
   「Ctrl+D to favorite」，而这一页没有任何一行可以收藏）——修它需要一条**新 i18n 文案**
   （去掉收藏半句的短版），踩 i18n 红线，先记进「需拍板」。
