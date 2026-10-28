# P11-aj 规格 · 界面上没有两段字压在同一块像素上

## 意图

把「同一张面上两段字互相压住」和「一段字跑出窗口」变成启动器与设置页两个表面、
两套外观、两个窗口尺寸上的**常驻 pin**——这两件事在文本树里都看不出来
（`Tester.Texts` 把被裁掉的行与被压住的行一样报出来），所以之前**没有任何测试**
守得住它们。本轮顺带把三表面（启动器 / 设置页 / 终端页）逐帧审计了一遍，结论写在
报告里：**没有压字，也没有出框**。

为什么现在做：用户 2026-10-28 的实测反馈要求「先优化界面、质感、交互逻辑」。选题
顺序里剩下的条目全部要先拍板（见 `.pending-bugs.md` 顶部「需用户拍板」），所以本轮
在「界面质感与交互」这一档里取了唯一**不需要拍板**、又能实际守住质感的一件事：
一行字压到另一行字，是这类界面上最刺眼、又最难被测出来的毛病（P11-ag 就是这样
交付了「一张只有一句话、那句话还被盖住的空白卡」）。

## 落地项

### 1. 启动器：`internal/launcher/textframe_test.go`

- 位置：新增文件，`TestTheLauncherPaintsNoTextOverText`。
- 现状：无。启动器有 100+ 条 pin，全部读 `HasText` / 单个矩形，**没有一条**量过
  「两个矩形有没有叠在一起」。
- 应改成：

  - 每帧从 `Tester.Texts()` + `Tester.Find()` 取每个文本节点的盒子；
  - 丢掉**整窗宽**的节点（`ResultsLabel` 这类面板/列表自己的无障碍标签，按构造
    包含每一个字，不是一段字）；
  - 丢掉**被另一个盒子完全包含**的节点（按钮的字在按钮里面，是一块面上一段字）；
  - 剩下任意两盒在两个轴上都重叠 > 1 DIP → `t.Errorf`；
  - 任意盒越出窗口边框 → `t.Errorf`；
  - 三个页面 × 两套外观：空页（recents）、有匹配（含一个会被省略号截断的超长
    应用名，行尾还有 `PATH` 键帽）、零匹配。

  夹具里的超长名字是**故意**的：行文字列与行尾控件相接的地方，正是「多 1u 就压上」
  的地方。

### 2. 设置页：`internal/settingsui/textframe_test.go`

- 位置：新增文件，`TestTheSettingsPagesPaintNoTextOverText`。
- 现状：同上，五页的 pin 全部是单矩形（`TestTheSettingsPagesPaintNoTextOverText`
  之前一条都没有），分组标题与首行之间只靠 `Gap` 保着，`Gap` 写错没有任何断言红。
- 应改成：同一条规则，五页（general / sessions / shortcuts / integrations / about）
  × 两套外观 × 两个窗口尺寸（`860×900` 主尺寸、`720×560` 最小尺寸）。

### 3. 合约：`docs/raycast-design.md` §6.16

记下这条不变量与它守的两个表面，并说明**为什么它不属于「像素细节」**：压字是
P11-ag 那类「整块面看起来是空的 / 一句话被另一块盖住」的同一族。

## 行为契约零改动红线

- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线：**零改动**。
- 产品代码本轮**一个字节不改**（这也是台账 V=0 / B=0 / S=0 的由来），只新增测试
  与文档。审计若抓到压字/出框，本轮应当改产品；实测没抓到，报告里写明「抓不到」
  而不是先把 pin 放宽。

## 门槛（用本轮实测值）

```
gofmt -l cmd internal                    # 空
go vet ./...                             # 净
GOOS=windows go vet ./...                # 净
GOOS=darwin  go vet ./...                # 净
go test -count=1 ./...                   # 全绿
--- PASS 计数                            # 2365（基线 2363）
--- SKIP 计数                            # 26
测试函数数                                # 1933（基线 1931）
GOOS=linux|windows|darwin go build ./... # 过
```

## mutation 计划（≥2 条，实跑）

- **M1**：`internal/launcher/launcher.go` 里 `listEdge := edge + chips` 之后加
  `listEdge = 0`——列表不再为字段行让出那条带，首行的标题与副标题直接压在字段的
  查询文字上。让 `TestTheLauncherPaintsNoTextOverText` 变红（浅色空页、匹配页两处）。
- **M2**：`internal/settingsui/rows.go` 的 `section` 里给行容器加
  `.Margin(-t.Space(9), 0, 0, 0)`——分组标题被向上拉 36u，压住本节第一行。
  让 `TestTheSettingsPagesPaintNoTextOverText` 变红（General / Shortcuts 两页）。
