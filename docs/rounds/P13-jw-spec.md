# P13-jw · 安装确认的当前声明与等待出口

意图：让已有安装入口在等待确认时只保留一个问题，回答只属于用户实际看过的声明。依据最新 P13-jv「仍待」推进；不重复 P13-hw 的旧线索。

读取为：面向桌面工具用户的集成设置与安装确认，克制的轻便液态玻璃；拨盘 3/2/7。三秒内能辨认等待中的入口、当前工具与可重试出口。复用现有忙态胶囊与通知，不增加页面、依赖或常驻提示。

## 落地项（动手前代码位置）

1. `internal/settingsui/settingsui.go:7453` / `internal/shell/shell.go:2927`：本地安装等待时仍可再次选目录 → 同入口显示已有忙态并拒绝重复派发，取消/失败后恢复。
2. `internal/settingsui/settingsui.go:2181` / `internal/shell/shell.go:2786`：推荐连接等待时仍可重复点 → 按推荐 ID 显示忙态，其他入口独立。
3. `internal/extensions/install.go:195` / `shell.go:2941`：本地确认后重新读取已变化声明 → 提交前比对审核摘要，失效答案不写盘。
4. 同一 Commit / `shell.go:2808`：推荐确认存在同源缺口 → 推荐真实 Click 与磁盘变更覆盖，旧回答不批准新权限。
5. `internal/extensions/install.go:68` / `:195`：本地审核后 package.json 改指向仍使用旧入口 → 复核当前解析路径、包名、版本。
6. 同一 Commit / 推荐路径：推荐审核后的包入口变化也能接上旧声明 → 同源复核并通过推荐入口验收。
7. `internal/shell/shell.go:2694`：深链取消仍调用 Commit(false)，期间权限被移除则可能安装 → 拒绝答案直接退出；无失败/成功通知。
8. `internal/shell/shell.go:894`：registry 取消同样可能落盘 → 取消直接释放暂存包并退出；不改重复 ID 覆盖规则。

每批两项；先注入/旧代码事件红，再修复并定向 test/vet 后 commit + push。报告按批追加。若事件不成立，不计为修复。

## 红线与设计值

高度链、键盘模型、会话管理、设置写回、i18n 文案、命令接线与 3000ms 确认窗口不改。安装重复 ID、重装事务与生成工具草稿归属仍待拍板。这里仅约束既有审核的答案归属，不增加权限系统。

读 qiaomu-design / 偏好账本 / craft-loop / preflight / 中文排版。Apple DESIGN 取单一交互蓝角色、8px 控件圆角、1px 边；映射本仓已有 token，不照搬网站色值。工艺菜单 #4：同入口在飞派发最多1次；#6：过期成功/失败提示0条；#12：忙态沿用既有受光边策略，透明面新增0层。为何更轻：等待状态留在原入口，旧确认不制造新卡片和误导通知。

## 门槛与变异

中文 locale `LC_ALL=zh_CN.UTF-8 GOMAXPROCS=4`。开盘完整 baseline 实测 **5974 PASS / 0 FAIL、exit0**。测试函数数量以开盘提交树核对（运行期间已经加入本轮测试，不能误算为开盘函数）。

全量一次：gofmt -l cmd internal；三平台 go vet ./...；go test -count=1 ./...；go test -count=1 -v ./... 计 PASS；三平台 go build ./...。每批仅定向回归与受影响包 vet，避免重复全量。

M1 恢复两入口可点击：重复 Click pin 红。M2 撤销摘要检查：本地/推荐新权限写入 pin 红。M3 撤销包路径检查：旧声明仍安装 pin 红。M4 恢复取消进入 Commit：取消后仓库变化 pin 红。每组保存原文件 MD5，实跑红、还原、md5sum -c、touch 复跑。深浅 ui.Tester PNG，比较图入轮次目录；原生系统权限弹窗与其他系统真机不冒充已验。
