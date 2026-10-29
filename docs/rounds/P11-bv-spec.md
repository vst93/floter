# P11-bv 规格 · 中文界面里不会再冒出一句英文（同字集合只许缩小）

## 0. 意图一句话

`Copy` 的 en / zh 两份之间**每一段显示文案都必须不同**，唯一允许同字的是「存进设置文件的 ID /
语言自己的名字 / 品牌名」这三类，外加一处已记账的用词债务；再加一处就红。

## 1. 为什么现在做（证据）

选题链：用户实测反馈 F1–F4 全收完（`P11-y`/`z`/`aa`、`bb`/`bj`/`bk`/`bo`）→ 合约
`docs/raycast-design.md` §6 全 `已落地`（只剩拍板项）→ `P11-bu` 仍待第 1 条整张是老拍板项、
第 2 条是尺子的边界、第 3 条卡在 `Element.Cache` 与上游 `actionSpinner`
→ 按用户定的下一档「界面质感与交互 → 基础功能」自己找，落在这条上：**用户读的是 zh**
（P11-ba 的原话），而 zh 里留一句英文正是"界面质感"里最刺眼的一种。

今天的守卫都不回答这件事：

- `internal/i18n/i18n_test.go:13` `TestEveryLanguageIsComplete` 只查 zh **不空**
  （`checkComplete` 判的是 `strings.TrimSpace(tr) == ""`，`:62-73`）；
- `internal/i18n/i18n_test.go:35` `checkNoEmptyStrings` 只查 en **不空**；
- `internal/i18n/deadcopy_test.go:27` `TestNoDeadCopy` 只查每个键**有人在读**。

三者都不看「zh 与 en 逐字节相同」。实测全表同字叶子 **52 条**（探针逐叶打印，见报告 §2），
其中 47 条是 ID、2 条是语言自名、1 条是品牌、2 条是真的英文显示词
（`Copy.Settings.AppIcons[0].Label` = `Dark`、`[1]` = `Light`，中文界面下也写英文）。

## 2. 逐条落地项

| 位置 | 现状 | 应改成 |
| --- | --- | --- |
| `internal/i18n/sharedwords_test.go`（新） | 没有这个文件 | 新测试 `TestOnlyIdentifiersAndNamesAreSharedByBothLanguages`：反射遍历 `en`/`zh`，收集同字叶子；按路径形状归类（`.ID` → ID、`.Languages[N].Label` → 自名、`Copy.Notifications.Title` → 品牌、`.AppIcons[N].Label` → 已记账债务），归不进四类的同字叶子报错 |
| 同上 | — | 遍历自证：同字叶子 < 47 条即判遍历漏分支（ID 一族实测 47 条） |
| 同上 | — | `AppIcons` 两条只 `t.Logf` 不报错，并指向 `.pending-bugs.md` 的用词拍板项 |

**产品代码零改动**：本轮只加一把尺子，翻译文案一个字没动（`Dark`/`Light` 是否改「深色/浅色」
是用词拍板项，见 `.pending-bugs.md`）。

## 3. 行为契约零改动红线

高度链、键盘模型、会话管理、设置写回、**i18n 文案**、命令接线：全部零改动。
`copy.go` 一个字节没动（变异后 `md5sum -c` 核对）。

## 4. 门槛命令 + PASS 基线（本轮实测的当前值）

```
gofmt -l cmd internal                                   # 期望空
go vet ./... && GOOS=windows go vet ./... && GOOS=darwin go vet ./...
go test -count=1 ./...
go test -count=1 -v ./... | grep -c -- "--- PASS"
GOOS=linux go build ./... && GOOS=windows go build ./... && GOOS=darwin go build ./...
```

基线（P11-bu 实测）：PASS **2559**、FAIL 0、SKIP **54**、测试函数 **2025**。
本轮 +1 函数 / +1 PASS。

## 5. mutation 计划（实跑，红 → 还原 → `md5sum -c`）

- **M1**（尺子对「漏译」的敏感度）：把 `zh.Launcher.Clear` 改成 en 的 `"Clear"`
  （`copy.go:1000`）→ 新测试必须红，且 `TestEveryLanguageIsComplete` 仍绿（证明它查不到这类）。
- **M2**（遍历是不是真的在走）：把 `sharedLeaves` 的 `reflect.Slice` 分支去掉 → 同字叶子
  只剩标量那几条，`len(shared) < 47` 的自证必须红。
