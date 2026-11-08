# P13-jw · 安装确认的当前声明与等待出口

开盘 2cca8ae9，pull 已同步，工作树干净。仓库已到 P13-jv，本轮按最新遗留推进。Codex 独立执行。开盘指定 done.log 不存在。基线与分批证据增量记录。

## Stash 清点

八条均为已审计早期独立 WIP，SHA 与上轮相同，无新增 stale-kill；逐条 reverse-check 退出1，不等于独有工作。沿用既有语义审核，不把旧主题/Tauri覆盖当前实现。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap，reverse=1 | 保留 |
| stash@{1} | 8af94a07 | 旧rows/slider，reverse=1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局，reverse=1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse=1 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽，reverse=1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse=1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse=1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧main/Tauri，reverse=1 | 保留 |

## 实施记录

待实际事件复现后逐条记现象→改动→验证。旧V/B/S断言保持0/0/0；不改旧断言、不新增Skip。

### 第一批：两类安装入口的等待态

1. 现象：本地安装审核未回答时，原目录入口仍可点击；深浅事件红串 `pending local-busy permits repeat: calls=1`。改动：本地入口在审核、安装、刷新期间使用既有忙态胶囊，并在宿主拒绝重复派发。验证：真实Click等待时派发0次，取消结束后原入口可再点1次。
2. 现象：推荐工具连接审核期间同样可重复点击，红串 `pending recommended-busy permits repeat: calls=1`。改动：按推荐ID占用其自己的入口，复用同一忙态材质。验证：深浅Click和直接重复宿主派发，审核仅1次，取消后恢复。

旧代码的四个深浅场景均实红，原始 before PNG 和 M1-before.txt 已生成。后续正式回注、MD5复原、门槛数字据实追加。

第一批相关回归 **53 PASS / 0 FAIL**，settingsui/shell vet净。M1回注两个原入口，四场景实红；两文件MD5逐字节还原，touch后 **6 PASS**。浅色图目视：推荐行“接管”原位出现等待环并退淡，本地“选择…”保持可用；没有遮罩或新卡层。完整基线仍运行中，提交只引用已完成的定向数字。

第一批 **18326336** 已推送。开盘全量完成：**5974 PASS / 0 FAIL，exit0**。

### 第二批：声明字节变化使旧回答退役

3. 现象：本地审核窗口打开后新增clipboard-read，确认仍安装并批准新增权限。改动：Prepared把审核摘要带到安装读盘边界，不一致退役；本地入口安静恢复。验证：真实选择Click→审核门控→磁盘变化→确认，仓库字节不变、旧通知0条，新审核仍可安装。
4. 现象：推荐连接审核后暂存声明改变，同样把旧回答用给新权限。改动：共用摘要保护，推荐入口处理失效结果并恢复。验证：推荐行真实Click及其独立暂存目录变化，深浅两套同验。

第二批相关 **86 PASS / 0 FAIL**，extensions/shell vet净。M2撤销摘要检查后四场景报 `obsolete local-permissions review rewrote repository` / `obsolete recommended-permissions review rewrote repository`；MD5还原、touch后 **6 PASS**。开盘树 `git grep -h '^func Test' 2cca8ae9 -- '*_test.go'` 实数 **3395**（上一轮报告写3393，本轮不沿用误差）。

第二批 **41dadd06** 已推送。

### 第三批：包入口变化使旧回答退役

5. 现象：本地审核时保留原manifest字节，只将package.json改指current.json，旧回答仍登记原声明。改动：提交前重新解析本地包入口、名称与版本；重新审核后记录实际manifest相对路径，避免读取旧文件。验证：真实Click→入口改指→确认，仓库不变；新审核成功且Inventory读到当前工具名。
6. 现象：推荐暂存包改入口也绕过摘要检查，仍登记旧声明。改动：推荐路径共用当前入口复核。验证：独立暂存包、深浅两套事件，失效回答不写盘/不发旧通知，重新点击推荐可正常安装原装包。

第三批原代码四场景均报 `obsolete local-package review rewrote repository` / `obsolete recommended-package review rewrote repository`。新审核后manifest路径也一并钉住，不单算第九项。

第三批相关 **92 PASS / 0 FAIL**，extensions/shell vet净。M3撤销入口复核，四场景实红；MD5还原、touch后 **6 PASS**。两次审核后的当前manifest名称有独立断言。
