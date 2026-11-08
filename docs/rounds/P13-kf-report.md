# P13-kf · 运行结果的阅读与退出恢复

开盘352f8cc4，pull已同步且工作树干净。Codex独立执行，未委派、未换模型。给定done.log开盘不存在。最新P13-ke仍待指向输出阅读；jw安装恢复已经后续轮完成，不重复。

## Stash清点

权威活动清单8条，无新增stale-kill。交接账登记为早期独立WIP；各项反向校验exit1，并非当前待接续抢救稿。保留原件，不将旧主题与Tauri发布链覆盖HEAD。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | 旧keycap/主题，reverse1 | 保留 |
| stash@{1} | 8af94a07 | 旧rows/slider，reverse1 | 保留 |
| stash@{2} | 7e4aa710 | 旧卸载布局，reverse1 | 保留 |
| stash@{3} | 1e3aec94 | 旧数字字段，reverse1 | 保留 |
| stash@{4} | 363f5a42 | 旧文本槽，reverse1 | 保留 |
| stash@{5} | 9b8ce23c | 旧快捷键，reverse1 | 保留 |
| stash@{6} | 90d314b2 | 旧动作栏，reverse1 | 保留 |
| stash@{7} | 7a9f6e13 | 旧main/Tauri发布链，reverse1 | 保留 |

## 增量实施

基线与事件复现进行中，只按实测补录。旧V/B/S断言目标0/0/0。

1. 展开后再次Enter无效（旧槽位复用造成看似持焦）→ 折叠入口显式身份，展开交接到原收起键 → 深浅可见断言与两次Enter往返通过。
2. 收起后查看入口失焦 → 收起键持焦时返还原查看入口并一次显现 → 深浅入口完整可见，Enter可立即重开。

第一批定向6 PASS / 0 FAIL，settingsui vet净、gofmt空。M1撤回本批产品实红：`disclosure cannot reverse with keyboard`与`run outlet outside viewport: label="查看输出" focused=false`；MD5还原、touch复验6 PASS。V/B/S旧断言0/0/0。完整开盘门槛尚在运行，不填写未完成的全量数字。

第一批87326334已推送。开盘完整日志最终6327 PASS / 0 FAIL，go test exit0，3477函数（包装命令末尾rg无FAIL匹配返回1，不是测试失败）。

3. 阅读中的结果被外部折叠后丢失焦点 → 输出面记录现有阅读焦点，折叠时回到原查看入口 → 深浅入口完整可见、Enter重新展开，输出原字节不变。
4. 正文被空结果替换后键盘落空 → 可选择正文退役时交接原收起键 → 深浅空态可退出；存储结果仍为空，不复制旧结果。

第二批新增场景6 PASS，连同第一批12 PASS / 0 FAIL，settingsui vet净、gofmt空。前图实红均为`run outlet outside viewport`且focused=false。首次夹具没有滚到正文，只记夹具错误；补真实阅读位置后才计复现。V/B/S旧断言0/0/0。M1已实跑，第二个正式mutation留给结果选区隔离。

第二批41da540a已推送。

5. 正在阅读的集成变为孤立包，结果面退役但焦点落空 → 按现有退役规则回到页面退出入口 → 深浅Close持焦且可见，Enter恰好关闭一次，历史字节保留。
6. 正在阅读的集成从列表移除，输出未登记退出路径 → 纳入现有missing-notice恢复登记 → 深浅Close持焦可用，原结果记录不删除。

第三批累计18 PASS / 0 FAIL，settingsui vet净。首次夹具漏接Actions.Close，不作为产品证据；接回真实页面出口后M2回注第二批产品，四场景实红`retired output lost page keyboard exit`，两产品文件MD5还原、touch后18 PASS。V/B/S旧断言0/0/0。

第三批4ee579c0已推送。

7. 新运行以前次结果为前缀或完全相同，复制会沿用前次选区 → 每次SetRunResult给予正文独立身份，当前读者焦点交接到新正文 → 深浅×相同/前缀四场景复制前残余选区为0；重新全选复制完整新结果，普通重绘保留当前选区。
8. 截断标记新增或撤回会改变头部槽位，收起键失焦 → 收起键使用独立稳定身份 → 深浅×标记增删四场景保留原焦点，Enter恰好收起。

第四批累计43 PASS / 0 FAIL（含14个用户已转移到Close的保护场景），settingsui vet净、gofmt空。M3回注第三批产品，八个场景实红`new run inherited previous result selection`或`run outlet outside viewport: label="收起输出" focused=false`；MD5还原、touch后43 PASS。旧V/B/S断言0/0/0。
