# P11-by 规格：「字压字 / 出窗」这把尺子——整宽句也下场，一份实现三处共用

## 1. 意图（一句话）

把「字压字 / 出窗」审计里**唯一被整类跳过的一档**（`r.W >= w-1` 的整宽文本框）放回审计，
并把它在三份拷贝（launcher / settingsui / terminalui）里的实现收成**一份**。

## 2. 为什么现在做（证据，本轮实测行号）

选题顺序：用户实测反馈 F1–F4 已全部落地（P11-y/z/aa、P11-bb/bj/bk/bo）→
`docs/raycast-design.md` §6 全 `已落地`（只剩拍板项）→ P11-bx「仍待」第 1 条整张是拍板项、
第 2 条正是**尺子自己的盲点**（上一轮实测产物）→ 按用户定的顺序落在界面质感档里
唯一不需要拍板的一件事：**上一轮自己写下的「别继续加字压字臂，要给整宽句单独一条臂」**。

上一轮的变异台账（`docs/rounds/P11-bx-report.md` §4）里有**两条产品缺陷实跑仍绿**，
它们都撞在同一处跳过上：

- **M4**：拒绝句 `FontSize(t.FontSize)` → `× 6`（句子放大六倍）**仍绿**。
  原因：`internal/settingsui/textframe_test.go:48` 的 `if r.W >= w-1 { continue }`
  —— 那条横幅本来就整宽，于是它**从来没被量过**。
- 同一处跳过在另外两个表面各有一份拷贝：
  - `internal/launcher/textframe_test.go:54`（`// the panel's/list's own label, not a run`）
  - `internal/terminalui/draweroverlap_test.go:86`（`// the strip's own label, the panel's, a whole row's box`）

三份拷贝的**跳过理由**是「整宽箱子是祖先，它按构造包含每一个 run」——理由本身成立，
但它是 `encloses` 已经覆盖的关系，**不需要另开一条跳过**；而这条跳过顺手把
「整宽叶子」（横幅、整宽段落）一起扔掉了。**一条为优化写的跳过，变成了一个盲点。**

## 3. 逐条落地项

### 3.1 一处实现：新 `internal/frameaudit`

| 位置 | 现状 | 应改成 |
| --- | --- | --- |
| `internal/launcher/textframe_test.go:33-113` | `auditLauncherFrame` + `encloses`/`minF`/`maxF`，整宽跳过 | 转发 `frameaudit.Audit`，删掉本地拷贝 |
| `internal/settingsui/textframe_test.go:26-91` | `auditSettingsFrame` + `enclosesBox`/`minBox`/`maxBox`，整宽跳过 | 转发 `frameaudit.Audit`，删掉本地拷贝 |
| `internal/terminalui/draweroverlap_test.go:56-130` | `auditDrawerBandFrom` + `encloses`/`minOf`/`maxOf`，整宽跳过 | 转发 `frameaudit.Audit`（`keep` 只留条带下的行） |
| `internal/terminalui/drawerband_test.go:65-66` | 自己算交叠 | 用 `frameaudit.Overlap` |

新包 `internal/frameaudit/frameaudit.go`（无 build tag，三平台都编译）：

- `type Run struct { S string; R ui.Rect }`
- `func Read(tt *ui.Tester) (w, h float32, runs []Run)` —— 一个字符串一个箱子
  （`Tester.Find` 按字符串索引，见 §4 的盲点）。
- `func Audit(t *testing.T, tag string, tt *ui.Tester, keep func(Run) bool)`
  - **两条断言，整宽箱子两条都参加**（本轮的实质改动）：
    1. 箱子必须在窗口内（`x >= -0.5 && y >= -0.5 && x+w <= W+0.5 && y+h <= H+0.5`）；
    2. 两个箱子在**两轴上都**交叠超过 1 DIP，除非互相包含（按钮的字在按钮里）。
  - 祖先关系仍由「互相包含」豁免 —— 整宽祖先本来就会被它豁免，所以删掉那条跳过
    不会引入它当初想挡掉的东西（实测见 §5）。
- `func Encloses(outer, inner ui.Rect) bool`、`func Overlap(a, b ui.Rect) (ox, oy float32)`

### 3.2 契约 pin

`internal/ci/frameaudit_contract_test.go`：源码扫描 —— `internal/launcher`、
`internal/settingsui`、`internal/terminalui` 里**不许再出现** `W >= w-1`（或变体的
`w - 1` 整宽跳过）与本地 `func encloses`，且三处都 import `floter/internal/frameaudit`。
（与 P11-i/P11-s/P11-l 的「一个读法」同一手法。）

## 4. 本轮**不**碰的（上一轮已写明的另两处盲点）

- `Tester.Find` 按字符串去重 ⇒ `want` 对「同一句话画两处」不敏感（P11-bx M2）。
  要守它得换一把能量「盒子存在」的尺子（框架不导出场景树），**不是本轮**。
- 尺子只量**字盒**，图标/面/描边与字的互压看不见（P11-bx M3）。同理留在「仍待」。

## 5. 门槛命令 + PASS 基线

```
gofmt -l cmd internal
go vet ./...
GOOS=windows go vet ./... && GOOS=darwin go vet ./...
go test -count=1 ./...
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

基线（本轮开工实测）：PASS **2595** / FAIL 0 / SKIP 54 / 测试函数 2029。

## 6. mutation 计划（≥2 条，每条说明该让哪个 pin 变红）

- **M1（应当红）**：`internal/settingsui/savealert.go` 的拒绝句
  `FontSize(t.FontSize)` → `FontSize(t.FontSize*6)`（= P11-bx M4 原件）。
  该让 `TestTheSettingsPagesPaintNoTextOverText` 与
  `TestTheSettingsPagesTransientFacesPaintNoTextOverText` 里带 `refused recording` 的臂变红。
- **M2（应当红）**：`internal/launcher/launcher.go` 里结果行标题 `FontSize` 放大到 6 倍
  （整宽/长句子那一行），该让 `TestTheLauncherPaintsNoTextOverText` 红。
- **M3（应当红，只在新规则下才红）**：给 `auditSettingsFrame` 的那个整宽箱子单独造一条
  「扩大后出窗」的臂——用 §3.2 的契约 pin 直接反向注入（把 `frameaudit` 的窗口内断言
  换成恒真），该让设置页/启动器/抽屉三处审计同时红。
- 每条注入 → 看到真红 → `git checkout` 还原 → `md5sum -c` 逐字节核对 → `touch` 后复跑。
