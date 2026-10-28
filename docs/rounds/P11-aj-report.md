# P11-aj 报告 · 界面上没有两段字压在同一块像素上

方向一句话：**把「压字 / 出框」这两个只能靠眼睛看出来的毛病，变成两个表面上的常驻 pin。**

选题顺序照 `.pending-bugs.md` 顶部：用户实测反馈 → 合约未落地条目 → 上一轮「仍待」
→「界面质感与交互」。前两档现在只剩**要拍板**的条目（`⌘K` 动作面、搜索框 caret、
终端滚动缓冲、默认浅色安静档 3.04:1……），本档里能找到的、**不需要拍板**的一件事
就是这条：整个仓里没有一条断言回答过「这一帧有没有两段字叠在一起」。P11-ag 交付
「一张空白卡」那种 bug 能出厂，正是因为 `HasText` 对**被盖住的**字与**看得见的**字
给出同一个答案。

## 1. 本轮实测：三表面逐帧审计，没有压字、没有出框

审计脚本（临时，未入库）走的就是本轮落地的那条规则，在下面这些帧上跑：

| 表面 | 帧 | 结果 |
|---|---|---|
| 启动器 | 空页 / 有匹配（含超长应用名）/ 零匹配 × 浅深 | 0 压字 · 0 出框 |
| 设置页 | general·sessions·shortcuts·integrations·about × `860×900`、`720×560`、`620×480` × 浅深 | 0 压字 · 0 出框 |
| 终端页 | 画布 + 浮条 + 抽屉 × 浅深 | 0 压字 · 0 出框 |

顺带否掉的三条（侦察结论，别当 TODO 重领）：

1. **设置页的滚轮**是真接上的：`tt.Wheel(360, 300, 0, 240)` 之后要**再跑 20 帧**
   （框架的滚轮滑行按帧推进），`Launch at startup` 从 y=482.6 走到 242.6；只跑两帧
   会读到「没滚」——那是测试写法，不是产品缺陷。
2. **换页会把页面滚动位置归零**（General 滚到底 `Body.Y=1062.1` → 按 ↓ 到 Sessions
   `Body.Y=0`），不会出现「新页面停在下半截」。
3. **`Esc` 两级**（有查询先清空、空查询再 `Dismiss`）与**输入法候选窗的落点**
   （`Tester.TextCaret` 在空查询 `{54,15.5,1,25}`、输入 `git` 后 `{76,15.5,1,25}`，
   都在字段里）都正确。

`Tester.Texts()` 里有一条 `{X:54 Y:446 W:14 H:24}` 的查询文本节点（渲染在 y≈16），
实测该位置**零墨**（y=440..470 的 x=50..130 区间暗像素计数 = 0），是框架量字阶段
留下的标签盒，不是画出来的第二份文字；本轮**不动**它（属框架侧，改它要动 mygo）。

## 2. 落地

- `internal/launcher/textframe_test.go`：`TestTheLauncherPaintsNoTextOverText` +
  `auditLauncherFrame`（三条判据：整窗宽节点不是字；被包含的盒是一块面上一段字；
  两轴重叠 > 1 DIP 才算压字）。
- `internal/settingsui/textframe_test.go`：同规则，五页 × 两外观 × 两个尺寸。
- `docs/raycast-design.md` §6.16：这条不变量入库。
- **产品代码零改动**，所以 V=0 / B=0 / S=0（没有旧 pin 需要重定、没有结构变化）。

## 3. 门槛（本轮实测）

```
gofmt -l cmd internal                    # 空
go vet ./...                             # 净
GOOS=windows go vet ./...                # 净
GOOS=darwin  go vet ./...                # 净
go test -count=1 ./...                   # 全绿，0 FAIL
--- PASS 计数                            # 2365（基线 2363，P11-ai 收尾实测）
--- SKIP 计数                            # 26
测试函数数                                # 1933（基线 1931；新增 2 条）
三平台 build                              # linux / windows / darwin 全过
```

## 4. 变异（2 条，实跑变红并逐字节还原）

**M1** —— `internal/launcher/launcher.go`，`listEdge := edge + chips` 之后加
`listEdge = 0`（列表不再为字段行让位）：

```
--- FAIL: TestTheLauncherPaintsNoTextOverText (0.08s)
    textframe_test.go:119: light empty: "Restart the computer" {X:54 Y:0 W:166 H:24}
        is painted over "Search" {X:54 Y:15.5 W:512 H:25} (overlap 166.0x8.5)
    textframe_test.go:119: light empty: "Asks first, then restarts" {X:54 Y:24 W:591 H:22}
        is painted over "Search" {X:54 Y:15.5 W:512 H:25} (overlap 512.0x16.5)
```

**M2** —— `internal/settingsui/rows.go` 的 `section`：行容器加
`.Margin(-a.cardTheme(c).Space(9), 0, 0, 0)`：

```
--- FAIL: TestTheSettingsPagesPaintNoTextOverText (0.17s)
    textframe_test.go:123: light general 860x900: "Window behaviour" {X:188.6 Y:414.89 W:653.4 H:19}
        is painted over "Launch at startup" {X:200.4 Y:417.8 W:587.6 H:18} (overlap 587.6x16.1)
    textframe_test.go:123: light shortcuts 860x900: "App keys" {X:188.6 Y:249.1 W:653.4 H:19}
        is painted over "New command" {X:200.4 Y:256.9 W:546.2 H:18} (overlap 546.2x11.2)
```

两条都用 `cp` 备份 → `md5sum` 逐字节核对还原（M1 `a0276a8a72ead04ab0f14b3eb42681bf`、
M2 `8554b4887b13cdf73af82049b65bd7b2`）→ `touch` 复跑绿，避免测试缓存复用。

## 5. 图上实测

非视觉轮（产品墨与面零改动，`FLOTER_PNG` 两套外观的帧与 P11-ai 收尾时逐像素一致），
不出图。审计的数字在上面第 1 节。

## 6. 仍待

1. **界面质感与交互 + 用户报的 bug 这一档，能做的只剩「要拍板」的四条**，请用户一次
   拍板（每条都只有一个问题）：
   - 搜索框 caret 是 `ui.Theme.Accent`（框架的，全仓最后一处常量红）；
   - 默认浅色外观的三档安静墨是 3.04:1（P11-aa 的拍板项）；
   - 终端「上次会话的滚动缓冲要不要回来」（要插件提供只含文本的导出）；
   - `⌘K` 动作面（页脚画着键帽，chord 没绑）。
2. 拍板清单其余部分原样有效（`.pending-bugs.md` 的「需用户拍板」表）。
3. 侦察结论（本轮实测否掉）：设置页滚轮 / 换页归零 / `Esc` 两级 / 输入法候选窗落点
   都正确；`internal/launcher` 与 `internal/settingsui` 的帧在 3 个尺寸上无压字无出框。
