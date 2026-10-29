# P11-bu 规格 · 把「实测干净」的两张表面变成常驻 pin

## 意图（一句话）

P11-bt 报告 §6.3 留下的两条探针（**文件页**与**插件配置表单**跑完即删）这次起正经文件名，
让「没有两段字压在同一块像素上、没有一段字跑出窗口」这条不变量在**用户能打开的最后两张
表面**上常驻；顺带把探针真跑一遍，红就是产品 bug，绿就是留下的理由。

## 为什么现在做（证据）

- 选题顺序：用户实测反馈 F1–F4 已全部收完（`P11-bj` / `P11-bk` / `P11-bo` 三条已推送）；
  `docs/raycast-design.md` §6 差距清单全 `已落地`（只剩拍板项）；P11-bt 仍待第 1 条是
  拍板项（搜索框 caret / 浅色 3.04:1 / 滚动缓冲 / `⌘K` / `AccentEdge` / 便条投影 /
  拖拽手柄 / `Permissions` 用词 / `configDiscardBar` 12u / 顶边失败带 / P11-bo 空页置入终端行），
  不动手；第 2 条（`Element.Cache` / `#753`）判据未定；第 4 条同上；第 5 条是上游缺口。
  **第 3 条明确写着「要变成常驻 pin 就起正经文件名（`filesframe_test.go` /
  `pluginsheetframe_test.go`）」**——本轮就是它。
- 现状（行号本轮重新核过）：
  - 审计尺子已存在：`internal/launcher/textframe_test.go:47 auditLauncherFrame`、
    `internal/settingsui/textframe_test.go:30 auditSettingsFrame`。
  - 已覆盖的表面：启动器三个普通页（`textframe_test.go:117 TestTheLauncherPaintsNoTextOverText`）、
    三个插件模式页（`internal/launcher/modetextframe_test.go:27 TestThePluginModesPaintNoTextOverText`）、
    设置页五页（`internal/settingsui/textframe_test.go:110`）、终端页（`internal/terminalui/*`）。
  - **没覆盖的两张**：
    1. 文件页 `internal/launcher/files.go:88 filesItems` —— 每条拖入文件三行、超
       `MaxVisibleDroppedFiles`(5) 收成一条「还有 N 个」行、行标题是
       `动作 + 文件名`（`files.go:127`），`Detail` 是目录（长路径）。这页的列表长度、行文案
       与普通结果页不同，从来没被量过。
    2. 插件配置表单 `internal/settingsui/pluginsheet.go:56 PluginSheet` —— 三种已发布 schema
       （`internal/plugincfg/schema.go:352 Clipboard`、`:382 Calculator`、`:441 Browser`），
       字段族最全（toggle / slider / select / number / radio / action），且是**唯一**在
       720u 启动器窗口里画设置语言卡片的表单（`internal/shell/pluginconfig.go:180`）。

## 逐条落地项

| # | 位置 | 现状 | 应改成 |
|---|------|------|--------|
| 1 | `internal/launcher/filesframe_test.go`（新） | 无 | `TestTheFilesPagePaintsNoTextOverText`：`lang∈{en,zh}` × `dark∈{false,true}` × 两个尺寸（`InputWindowWidth×frameHeight(maxResultSlots)`、`InputWindowWidth×360`）× 四态（未拖入 / 7 个文件收起 / 展开 / 输入不匹配的 needle），每态调 `auditLauncherFrame` |
| 2 | `internal/settingsui/pluginsheetframe_test.go`（新） | 无 | `TestThePluginSheetPaintsNoTextOverText`：`lang∈{en,zh}` × `dark∈{false,true}` × 两个尺寸（720×900、720×560）× 三种已发布 schema（Clipboard / Calculator / Browser），每态调 `auditSettingsFrame` |
| 3 | 两张表的覆盖自证 | 无 | 每张表里各一条「审计确实画出了东西」的臂（文件页要有会省略号截断的长文件名、表单要有每个字段族的标签），否则「没有压字」可能是「什么都没画」 |

## 行为契约零改动红线

- **产品代码零改动**（本轮只加测试）。若探针变红 → 先判是不是产品 bug，是则改产品并把
  红串抄进报告；不得为了绿而放宽审计规则（不加容差、不跳过、不删断言）。
- 高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线一律不碰。

## 门槛命令 + PASS 基线（P11-bt 实测）

```
gofmt -l cmd internal                                  空
go vet ./...                                            净
GOOS=windows go vet ./...                               净
GOOS=darwin  go vet ./...                               净
go test -count=1 ./...                                  0 FAIL
go test -count=1 -v ./... | grep -c -- "--- PASS"       2467（基线）
... | grep -c -- "--- SKIP"                             54
grep -rhn '^func Test' internal/ cmd/ --include=*_test.go | wc -l   2021
GOOS=linux / windows / darwin go build ./...            三份过
```

## mutation 计划（≥2 条，各自让哪条 pin 变红）

1. **M1（文件页）**：把 `files.go` 的「还有 N 个」行的 `Title` 改成 `copy.FilesMore(hidden) + copy.FilesMoreHint`
   （一条本来分两行的长字塞进同一段）→ 预期 `TestTheFilesPagePaintsNoTextOverText` 的
   `more` 臂报 overlap 或 leaves the window。若这条不红，改注入 `fileRows` 的
   `Title: row.action + "  " + file.Name` → `Title: row.action + "  " + file.Name + "  " + detail`
   （同一段字吃掉行尾的键帽/详情）。
2. **M2（插件表单）**：把 `pluginsheet.go` 的字段行 `Gap(t.Space(2))` 改成负值
   （例如 `.Gap(-t.Space(2))`）→ 相邻字段的标签压在上一行 → 预期
   `TestThePluginSheetPaintsNoTextOverText` 报 overlap。若 `Gap` 不接受负数，
   改注入 `sheetField` 里 label 列的宽度 `t.Space(...)` 归零。
3. **M3（覆盖自证）**：把 `MaxVisibleDroppedFiles` 临时改成 50 → 「还有 N 个」行消失，
   覆盖臂应当报「没有画出 more 行」；用来证明审计不是空跑。

每条：注入 → 看到红 → `md5sum -c` 逐字节还原 → `touch` 复跑绿。
