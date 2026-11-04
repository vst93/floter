# P13-ga 报告：键盘焦点的环 —— 20 个可聚焦站点，环画在盒子外 3px

**意图**：P13-fi 数完了「能不能点」（14 个 `Disabled(...)`、一套光标）。这一轮数「键盘走到
哪里看不看得见」：`Focusable()` 在 `internal/` 里 **20 处**，**没有一处**自己画环，环全是框架
`ui/paint.go:385` 给的。落地 = **零像素 + 两条 pin**：一条读画面（几何 + 颜色），一条扫 AST（普查）。

**这轮为什么让它更轻、更有空间感**：键盘焦点是唯一一种**不占任何像素**的层级——
它只在「这一块现在归键盘」的那一刻出现，出现的位置在盒**外**（3px），盒子本身一个像素都不动。
这正是轻便液态玻璃想要的那种层级：靠**关系**（谁在浮、谁被指向）而不是靠**加厚**（再加一圈边）。

## 1. 先量（探针并入 pin，数字见 `P13-ga-spec.md` §2）

- 带在 `[X-3, X-1)` / `[Y-3, Y-1)`，右侧与下侧镜像：**2px 宽、离盒 3px 起、盒沿本身不是环**。
- 浅 `(28,84,159)` / 深 `(123,158,227)`，与调色板 `RGBA(6,74,166,0.82)` / `RGBA(122,167,255,0.78)`
  压在中灰上的合成值**逐通道一位不差**。
- 盒内（内缩 2）逐像素不变；3u 带外逐像素不变。
- 圆角弧在四个**角顶点**侵入盒内 2px（弧心 (X+6,Y+6)、半径 9 过 (X,Y)）。
- 普查 12 文件 **20** 站点、**0** 处 `FocusRing(false)`。

## 2. 落地

| # | 文件 | 内容 |
|---|---|---|
| 1 | `internal/settingsui/focusring_test.go`（新） | `TestTheKeyboardRingRidesOutsideTheBox`（浅/深两臂：带是蓝的 8 处、盒内内缩 2 不动、3u 带外不动） |
| 2 | 同上 | `TestNoOneTakesTheKeyboardRingAway`（12 文件 → 20 站点的表；`FocusRing(false)` 必须同函数手画环） |
| 3 | 同上 | `drawsOwnRing`（框架的规则：`FocusVisible` 时用 `Painter.FocusRing` 自己画） |
| 4 | 同上 | opt-in dump `TestDumpP13GaFocusRing`（`FLOTER_P13GA_DUMP=1`） |
| 5 | `docs/liquid-glass.md` | 新增 **§6.146**（含 146-1…146-4） |

**产品代码零改动**：`git diff --stat` 只有新测试文件与 `docs/`。视觉已经是对的——
这一轮的价值是「它以后还会是对的」这句话第一次有了 pin。

## 3. 台账与门槛（实测）

**V 0 / B 0 / S 0**：没有一条既有 pin 变红，没有一条断言被改，产品零改动。

```
gofmt -l cmd internal                     # 空
go vet ./... && GOOS=windows go vet ./... && GOOS=darwin go vet ./...   # 净（VET_OK）
GOOS={linux,windows,darwin} go build ./...                                # 过（BUILD_OK）
go test -count=1 ./...                    # 全绿，0 FAIL
PASS 计数 3453（基线 3449，+4 = 2 个函数 + 2 个子测试）
测试函数 2613 → 2616（+2 pin，+1 opt-in dump）
```

## 4. 变异（实跑，原始报错串照抄）

- **M1** `internal/theme/theme.go:757` `ui.RGBA(6,74,166,0.82)` → `ui.RGBA(128,128,128,0.82)`：
  ```
  --- FAIL: TestTheKeyboardRingRidesOutsideTheBox/light
      focusring_test.go:131: left band at X-3 (17,40): the ring is not blue (r=128 b=128, gap 0 < 60)
      focusring_test.go:131: left band at X-2 (18,40): the ring is not blue (r=128 b=128, gap 0 < 60)
      focusring_test.go:131: top band at Y-3 (80,17): ... （四条边共 8 行红）
  ```
  `git checkout -- internal/theme/theme.go` → `md5sum -c` 逐字节核对（`4789e242…`）成功。
- **M2** `internal/settingsui/rows.go:1807` 的 `Focusable()` 后加 `.FocusRing(false)`：
  ```
  --- FAIL: TestNoOneTakesTheKeyboardRingAway
      focusring_test.go:258: rows.go:1: `FocusRing(false)` with no hand-drawn ring
  ```
  `git checkout --` → `md5sum -c`（`11fdeeef…`）成功。
- 两次还原后 `touch` 两个文件再跑一遍（避开测试缓存）：`ok floter/internal/settingsui`。

## 5. 偷了什么（`qiaomu-design` skill）

- 从 `references/craft-loop.md` **工艺密度菜单第 3 项**「品牌 focus 环：focus-visible 用品牌色
  **+ 偏移**，不用浏览器默认蓝」——本轮量出来的两个数正是这一项的两个半边：
  **色** `RGBA(6,74,166,0.82)` / `RGBA(122,167,255,0.78)`（复用调色板，不是框架 stock 的天蓝），
  **偏移** 3px + 2px 宽（框架 `paint.go:1353-1362`，本轮第一次钉进 pin）。
- 从 `references/user-preferences.md` **P-5「可读性优先于气质」**取判据形式：不写「环要有品牌色」
  这种散文，而是写一条能从画面读出的阈值（B−R ≥ 60），并给出两档的实测合成值。

## 6. preflight（逐条，`references/preflight.md`）

- **A 反 AI 套路**：过。零像素改动，没有新增装饰、渐变、光晕；环是原生控件语义，不是风气件。
- **B 设计读取一致性**：过。环的色来自唯一来源 `theme.go:757/:880`，实测与合成值逐通道一致；
  两档 focus 是**不同值**，与深浅两套地面的对比关系各自成立（M1 证明 pin 读的是这个来源）。
- **C 动效抽查**：过（有条件说明）。环没有过渡帧——框架在 `paint` 里即画即读，所以「按下 Tab
  的下一帧就有环」；本轮没有慢放逐帧的可选路径（静态导出），故这一项按「无动效可抽查」记为过。
- **D2 中文页**：过（不适用）。本轮没有任何文案、字号、行高改动。

## 7. 仍待

1. **下一格首选仍是 `140-5` 需拍板**（同一条顶边在深浅下读成相反的东西）；替代路径 = 通用队列
   里还没专轮过的面。
2. **新记一条量出来的事实（防误判）**：环在盒**外**——所以任何 `Clip` 与盒沿齐平的祖先都会
   把环整条吃掉。本轮只量了「孤零零的一枚控件」，**没有**量 20 个站点各自离裁切祖先多远；
   要量就得逐站点造帧，属下一轮的活（如果谁要把某块面板改成 `Clip`，先看这条）。
3. 需拍板（沿用）：`tokens.Caption` 9.9px / 正文 10.8px 与 preflight D2「正文 ≥ 14px」。
4. 需拍板（沿用）：启动器两行结果行 36px 盒装 27 行墨；`ui/tooltip.go:133` 禁用元素不弹提示。
