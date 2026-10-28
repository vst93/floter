# P11-ah 报告 · 滚轮滚不动结果列表：指针悬停的选中把平滑滚动掐断

规格 `docs/rounds/P11-ah-spec.md`。选题顺序：用户实测反馈（F1/F2/F3 已由 P11-y/z/aa 收完）
→ 合约未落地条目（§6 全表 `已落地`，只剩 `⌘K` 动作面＝拍板）→ 上一轮「仍待」第 1 条（同上）
→ **界面质感与交互逻辑**。本轮在这一档里找到的是一条**真交互 bug**，不是像素细节。

## 1 现象与根因

鼠标滚轮在启动器结果列表上一格只挪一行（120u 该走三行），而且是瞬跳、没有滑行。

根因链（全部本轮实读代码 + headless 实测）：

1. R21「指针进入某行就选中它」（`launcher.go:2156-2165`，参考
   `main:src/launcher/LauncherResults.tsx:859-862` 的 `onPointerEnter`）在实现上读的是
   「指针底下那一行**变了**」——滚轮把内容从指针底下挪走时它也成立。
2. 选中一变，`launcher.go:1465` 的 reveal 段就发 `List.ScrollIntoView(row)`。它本来是
   **键盘**的规则（注释原文：「Keep the chosen row in view when the keyboard moved the
   choice」），对指针刚进入的行没有意义（那行就在指针底下，`near` 语义下它本来也不会
   移动内容——`mygo@v0.7.0/ui/list.go:1383-1400`）。
3. 框架把任何一次 `ScrollIntoView` 读成「应用接管了偏移」：`ScrollIntoView` 无条件置
   `ListState.scrolled`（`ui/list.go:303-306`），而滚轮的滑行在自己的循环里见到它就退出
   （`ui/wheel.go:88-96`：`s.list != nil && s.list.scrolled` → `s.wheel = nil`）。

于是每一格滚轮只走掉「被丢弃前的那一小段」。**不是滚轮没接上**（触控板路径一直是好的，
`scroll_test.go` 只钉了触控板那条），是滚轮的滑行被自己的 reveal 请求掐死。

实测（31 行结果、窗口十行、指针停在列表中段、四格各 120u 合计 480u）：

| | `List.Visible()` | 走了 |
|---|---|---|
| 改前 | `[0 9]` → `[4 15]` | 4 行 |
| 改后 | `[0 9]` → `[10 21]` | 10 行 |

## 2 改法

新增 `App.pointerChose`（`internal/launcher/launcher.go:286-298`）：这一帧的 `Selected`
变化是不是**指针**给的。三处指针直写 `Selected` 的地方各置一次——行进入
（`:2160`）、收藏星点击（`:2394`）、行点击（`:2444`）；reveal 段先读后清
（`:1465-1490`），指针给的选择不滚。`a.chosenRow` 照旧推进，键盘步进 / 查询变更后的默认
选中 / 历史删除后的邻居接管全部不受影响。

红线未动：R21 的指针进入接管选中、R67 的持行不滚、键盘步进滚进视野、查询变更回顶、
高度链、字段键位、会话与设置写回、i18n 文案。

## 3 台账 V / B / S

- **V = 0**：没有旧 pin 钉过滚轮位移或滚轮手感。
- **B = 0**：`internal/launcher` 既有 pin 零改动全绿（`pointerselect_test.go` 的 R21 两条、
  `filters_test.go:1121` 的持行不滚、`scroll_test.go` 的触控板）。
- **S = 0**：无结构变更，只多一个状态位。

新增 3 条函数（2 pin + 1 opt-in dump）：`TestTheWheelScrollsTheListUnderThePointer`、
`TestTheKeyboardStillScrollsItsRowIntoView`、`TestDumpP11AHWheelPNGs`
（`internal/launcher/wheelscroll_test.go`）。

## 4 mutation（实跑，逐条变红后还原）

- **M1** reveal 段忽略 `pointerChose`（回到旧行为）：
  ```
  --- FAIL: TestTheWheelScrollsTheListUnderThePointer
      wheelscroll_test.go:65: four notches of 120u moved the list 4 rows (visible 4..0), want about 12: a notch is being cut short
  ```
- **M2** 删掉 `a.pointerChose = false` 那一行（标志永不复位）：
  ```
  --- FAIL: TestTheKeyboardStillScrollsItsRowIntoView
      wheelscroll_test.go:135: the choice is row 22 while the list shows 0..9: the keyboard's row was not scrolled into view
  ```
- 两次均 `cp` 还原 + `md5sum -c` 逐字节核对（`internal/launcher/launcher.go: 成功`）+
  `touch` 后复跑绿。

## 5 门槛（实测）

- `gofmt -l cmd internal` → 空
- `go vet ./...` / `GOOS=windows go vet ./...` / `GOOS=darwin go vet ./...` → 三份净
- `go test -count=1 ./...` → 全绿；**PASS 2360**（基线 2358）· SKIP **25**（基线 24，新增的
  是 opt-in dump）· FAIL 0 · 测试函数 **1927**（基线 1924）
- `GOOS=linux/windows/darwin go build ./...` → 三平台过

## 6 图上实测

`/tmp/floter-P11-ah-wheel-{rest,scrolled}-{light,dark}.png`（两格滚轮前后各一张，两外观）。

我在这四张图上看到的一处变化：浅色 `rest` 帧列表的第一整行是 `AppA0`，`scrolled` 帧同一列
的第一整行变成 `AppC0`——同一块面板、同一行几何，只有内容上移；深色帧逐点对应。改前同样
两格只挪一行（今天同一组四格的实测是 4 行，改后 10 行）。

## 7 仍待

1. **`⌘K` 动作面**仍是合约里唯一未落地的条目（§4.6）：页脚画着键帽但 chord 没绑定。要新增
   表面 + 新 chord → **需拍板**（原样有效）。
2. **需拍板（本轮新增，有实测）**：指针**静止**停在列表上时，键盘上下步进会被「滑到指针
   底下的那一行接管选中」吃掉一半——15 次 Down 只前进 8 行（`Visible()` 同步被拉回）。
   要修就得改 R21 的读法：「指针没动、是内容在动」算不算一次进入。参考侧
   （`LauncherResults.tsx:859` 的 `onPointerEnter`）在这个问题上不可判（React 的
   pointerenter 在内容滑过静止指针时是否触发取决于引擎），所以按纪律不擅自改。
3. 拍板清单原样有效（P11-aa 的默认浅色安静档、P11-z 的搜索框 caret、P11-y 的滚动缓冲、
   P11-ae 的 `CanvasAlpha`、P11-af 的浮条拖拽手柄、P9-g 表那一批）。
4. 侦察结论（别当 TODO 重领）：`internal/launcher` 的 Home/End/PageUp/PageDown **故意**没有
   绑定——参考的 `main:src/hooks/useAppKeyboard.ts` 只在单选组与输出页用它们，启动器结果
   列表只认 Up/Down；`scroll_test.go` 的触控板 pin 保留；输出页的滚轮由 `wheel_test.go` 守着。
