# P13-ho 报告：配置草稿与字段生命周期

开盘 ba5e3220，pull 最新，树干净。中文 locale 实测基线 3868 PASS / 0 FAIL / 2824 测试函数。
done.log 在本机不存在，收尾建立。qiaomu-design 的 SKILL、偏好、craft-loop、preflight、
chinese-typography 与 Apple 供体已读。未调用外部模型或子代理。

## 抢救 stash 清点

八条 reverse-check 均退出 1，与 hn 的 SHA 相同；无 stale-kill 新稿。
沿用 hn 已有语义审核：独立早期 WIP 不直接覆盖现实现，不把检查失败说成独有可合入。

| ref | SHA | 判断 | 处置 |
| --- | --- | --- | --- |
| stash@{0} | 422483b4 | P7 keycap 未完稿，非抢救 | 保留原件 |
| stash@{1} | 8af94a07 | 旧行距/滑块稿，非抢救 | 保留原件 |
| stash@{2} | 7e4aa710 | 旧卸载度量/探针，非抢救 | 保留原件 |
| stash@{3} | 1e3aec94 | 旧数字字段稿，非抢救 | 保留原件 |
| stash@{4} | 363f5a42 | 旧文本槽稿，非抢救 | 保留原件 |
| stash@{5} | 9b8ce23c | 旧快捷键形状稿，非抢救 | 保留原件 |
| stash@{6} | 90d314b2 | 旧动作栏行为稿，非抢救 | 保留原件 |
| stash@{7} | 7a9f6e13 | 旧 Tauri 发布稿，非抢救 | 保留原件 |

## 侦察纠正

“schema 放宽后数字保存值可能仍为 nil”未成立：数字 Changed 保存原始有限数，
只解析失败才写 nil。该线索撤回，不计修复。后续只凭真实事件红绿日志记账。

## 第一批（1–2）

1. 现象：文本改走再改回原路径，表单已显示 Saved，外部新路径仍被旧 live 遮住。
   改动：只在新宿主代际到来时，退役与 saved 同值且合法的编辑。验证：深浅真实输入、复制读到 external，焦点保留；真正本地编辑不覆盖。
2. 现象：数字改回 5.0 已与保存值 5 相同，外部 40 仍显示 5.0。
   改动：按有效数值识别撤回，合法同值才释放。验证：深浅复制得到 40，继续输入 12 遇外部刷新仍保留。

M1 回退 config.go 实跑：`reverted text masks host update: "/workspace/initial", want "/workspace/external"`；
`reverted number masks host update: "5.0", want "40"`。恢复 md5sum OK，touch 后相关回归 50 PASS / 0 FAIL，settingsui vet 通过。
V/B/S 0/0/0，旧测试零修改；新增 2 函数，截图 before/after 深浅均生成。

## 第二批（3–4）

3. 现象：数字输入 1e，schema 移除字段又重新加入，旧半输入和解析错误一起复活。
   改动：移除字段时退役 live 与其错误，保留已有 values 保存语义。验证：重加后显示空值，错误消失，可输入 8。
4. 现象：外部写入已与本地路径相同，状态仍显示 Unsaved changes，下一代路径仍被遮住。
   改动：仅当新宿主代际与合法编辑同值时确认 saved、释放 live。验证：Saved 恢复，下一代 latest 可见。

M2 原文：`removed number resurrected partial text: "1e"`、`removed number resurrected old parse error`、
`host-confirmed edit still says Unsaved changes`、`host-confirmed edit masks next generation: "/workspace/shared"`。
两项深浅实跑红，md5sum OK、touch 后相关 56 PASS / 0 FAIL，vet 通过。V/B/S 0/0/0。
