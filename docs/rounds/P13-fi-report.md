# P13-fi 报告：禁用面 —— 十四个 `Disabled` 调用点，一套说法

**意图**：任务书把「禁用 / 只读面」列为状态反馈的另一半，入口是 `rows.go:568-588` 那枚
「建时就禁用」的胶囊（注释里写着 `.Disabled(...)` 放在建好之后太晚）。本轮把它量到底：
**材质已经是一套说法**（P13-s/t/u/aq 那一代做完了），**指针那一半从来没数过**，
而「一套说法」本身还只是散文——没有一个东西守着新站点归到哪一档。落地因此是
**零像素 + 一条可执行的普查**。

## 1. 先量（探针跑完即删，数字抄在这里）

真设置页地面（`dimProbe`），同一行 ON = 可用 / OFF = 不可用；
`crown` = 顶内线 − 面，`shade` = 底内线 − 面，`ink` = 墨/面 对比级：

| 控件 | 外观 | 面 ON → OFF | crown | shade | ink |
|---|---|---|---|---|---|
| 动作胶囊 `disabledActionButton` | 浅 | 230 → **238** | +6 → **+0** | −14 → −7 | 202 → **105**（0.52） |
| 同上 | 深 | 39 → **26** | +19 → **+0** | −8 → −3 | 198 → **106**（0.54） |
| accent 主 CTA | 浅 | 82 → 164 | +41 → **+0** | 0 | 168 → 43 |
| 同上 | 深 | 193 → 104 | +5 → **+0** | 0 | 170 → 40 |
| 图标格（透明面） | 浅 | 247 → 247 | 0 | 0 | 138 → **69**（0.50） |
| 同上 | 深 | 14 → 14 | 0 | 0 | 141 → **70**（0.50） |
| 开关（槽） | 浅 | 79 → 161 | +1 → +1 | 0 | 175 → 48 |
| 同上 | 深 | 187 → 103 | 0 | 0 | 68 → 77 |
| 组件卡（面） | 浅 | 230 → **238** | +6 → **+0** | −14 → −7 | 202 → **105** |
| 同上 | 深 | 39 → **26** | +19 → **+0** | −8 → −3 | 198 → **106** |

三句话，正好是 §1 表里的三档：**面**离地面 9 级（浅）/ 12 级（深）、底对侧还在、顶内线 **+0**；
**槽**内线不动、整块退一半；**透明面**面就是地面、只有墨退一半。**没有一处需要改。**

指针（`ui.Tester.Cursor()`，真 hover 六种形状）：五处 `CursorDefault`，一处
`CursorNotAllowed`（卸载对话框里那张锁着勾选框的固定行卡，参考
`ComponentizedUninstallDialog.css:60-83` 的 `:has(input:disabled)`）。两个**不写** `Cursor`
的站点（`slider` 的轨道、`detachedBar` 的键）量过：两个 widget 自己都不设光标，解析器
（`mygo@v0.7.3/ui/runtime.go:1286-1324`）本来就给默认箭头——写了是把一句真话写两遍。
`updateCursor` 不看 `flagDisabled`，所以「禁用后指针该说什么」确实要每个站点自己回答；
本仓的答案是：**不说**，除了那一张卡。

## 2. 落地

| # | 位置 | 内容 |
|---|---|---|
| 1 | `internal/settingsui/pointercensus_test.go` | `disabledSites`：14 个真调用点（`file:func` + 四档 面/槽/透明面/根） |
| 2 | 同上 | `TestEveryUnavailableControlIsClassified`：扫 `internal/settingsui` 与 `internal/shell` 的 **AST**，每个 `X.Disabled(...)`（含 `Children(func(){…})` 闭包里的）必须在册；登记了却不再调用也红 |
| 3 | 同上 | `TestAnUnavailableControlPointsAtItselfWithOneOfTwoShapes`：六种形状 hover 实测 |
| 4 | 同上 | opt-in dump `TestDumpP13FIDisabledSheet`（`FLOTER_P13FI_DUMP=1`） |
| 5 | `docs/liquid-glass.md` | §1「禁用的材质」补「指针那一半」一段；新增 **§6.130**（含 130-1…130-7 台账） |

产品代码**零像素改动**：`git diff --stat` 里只有 `internal/settingsui/pointercensus_test.go`
与 `docs/`（本轮 spec、report、`liquid-glass.md` 三份）。

## 3. 台账与门槛（实测）

**V 0 / B 0 / S 0**：没有一条既有 pin 变红，没有一条断言被改；产品零改动。

```
gofmt -l cmd internal                       # 空
go vet ./...                                # 净
GOOS=windows go vet ./...                   # 净
GOOS=darwin  go vet ./...                   # 净
go test -count=1 ./...                      # 全绿（0 FAIL）
go test -count=1 -v ./... | grep -c -- "--- PASS"   # 3408（基线 3406，+2）
--- SKIP                                    # 191（基线 190，+1 = 本轮 opt-in dump）
测试函数                                    # 2577（基线 2574，+3）
GOOS=linux/windows/darwin go build ./...     # 全过
```

### mutation（实跑，原始报错抄在下面）

**M1** — `rows.go` 的 `pageButton` 里注入 `button.Disabled(true)`（一个 §1 表没点名的站点）：

```
--- FAIL: TestEveryUnavailableControlIsClassified (0.08s)
    pointercensus_test.go:108: rows.go:pageButton: pageButton calls Disabled(true) at rows.go:545:2,
    and §1's table does not name this site — add it to disabledSites with the arm it draws (面/槽/透明面)
```

**M2** — `uninstallPartRow` 固定行的 `Cursor(ui.CursorNotAllowed)` 改成 `Cursor(ui.CursorDefault)`
（指针的两句话并成一句）：

```
--- FAIL: TestAnUnavailableControlPointsAtItselfWithOneOfTwoShapes (0.23s)
    pointercensus_test.go:290: hovering the uninstall dialog's fixed component card shows 0, want 8
```

两条都 `git checkout` 还原，`md5sum -c` 逐字节核对（`11fdeeef…` / `52d1e032…` 两个文件都「成功」），
`touch internal/settingsui/*.go` 后复跑整个包：`ok floter/internal/settingsui 280.694s`。

## 4. 出图

`FLOTER_P13FI_DUMP=1 go test ./internal/settingsui -run TestDumpP13FIDisabledSheet` →
`/tmp/floter-P13-fi-disabled-light.png`、`/tmp/floter-P13-fi-disabled-dark.png`（560×320，
左列可用 / 右列不可用，五对：动作胶囊、accent 主 CTA、图标格、开关、组件卡）。

**这张图上看到的一处变化**（降采样成 70×40 灰度再看）：深色那张第 8–10 行，左列胶囊的顶行是
`*+++*`（一条亮线横在面上），右列同一行是 `-----`（面在、线没了）——受光边熄掉之后，
禁用面靠的是自己离地面那一格台阶，而不是一根更暗的描边。这正是本轮要守住的那句话。

## 5. 「偷了什么」（qiaomu-design / craft-loop.md 工艺密度菜单）

| 偷的项 | 落成哪个具体数值/动作 |
|---|---|
| 第 **12** 项 材质细节（内边框高光） | 「受光边是有/无」= 顶内线 **+6 / +19 / +41 → +0**，不是一个更淡的高光；四档表按「有没有受光边可熄」分家（面熄灯 / 槽不动 / 透明面只剩墨） |
| 第 **5** 项 多层染色阴影 | 对侧那一笔随面一起退一半：shade **−14 → −7**（浅）、**−8 → −3**（深）——禁用的面仍然站在 z 上，只是不发光 |
| 第 **6** 项 编辑细节（编号 / hairline / 大写字距标签） | 把「编号」用在账上：14 个站点 × 4 档，写成 `file:func` 的登记表，新站点不登记即红 |
| 第 **4** 项 签名交互（一个足矣） | 全仓只有一个「禁用还要说话」的指针形状（那张锁勾选框的卡），其余一律不说——签名动作只能有一个 |

## 6. preflight 逐条（A / B / C / D2）

- **A 反套路**：本轮不产生像素。无渐变、无发光、无纯黑、无斜体（`noitalic_test.go` 仍绿）、
  无三等分卡片、无「革命性/赋能」类文案（本轮**零文案**）。
- **B 设计读取一致**：读取为「桌面命令面板的设置页 / 集成页，面向每天用键盘的人，
  用轻便液态玻璃的语言」；记忆点未变。本轮的判据只有一条：**一个状态一个说法**——
  材质四档 + 指针两句，均可数。
- **C 动效抽查**：不动效。指针过渡仍是 `theme.ColorStep` 120ms，`ReduceMotion` 在探针里是开的，
  没有 `transition: all` 可违反。
- **D2 中文页**：本轮**不新增一个汉字**（i18n 文案是红线）。因此没有中西文空格风险；
  唯一的 D2 缺口仍是既有账：`tokens.Caption` 9.9px / 正文 10.8px 对「正文 ≥ 14px」，
  这是**需拍板**项（§6 已记），本轮不动。
- **A4.1（Carbon 决策门）**「disabled / read-only / hidden 语义没有混淆」：本轮把这一条
  变成了机器可查的——四档登记 + 指针两句；`read-only`（锁定字段）与 `disabled` 在
  `lockedControl` 的注释里是同一道淡、材质由调用点给，与 §1 表一致。

## 7. 仍待

1. **空态密度的最后一格**（通用队列「空态满态」里只剩空态）：下一轮候选。
2. 需拍板（沿用）：`tokens.Caption` 9.9px / 正文 10.8px 与 preflight D2「正文 ≥ 14px」。
3. 需拍板（沿用）：启动器两行结果行 36px 盒装 27 行墨；`ui/tooltip.go:133` 禁用元素不弹提示
   （禁用的「为什么」目前只有两处有字：透明度滑块的 `disabled` 原因句、卸载对话框的提示行；
   其余站点没有说原因，补一句就是加文案 = i18n 红线，所以只记录，不动）。
