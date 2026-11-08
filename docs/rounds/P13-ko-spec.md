# P13-ko · 插件配置刷新后的可见编辑出口

意图：宿主插入失败提示、刷新分组或选项后，当前键盘控件仍完整可见。实际开盘09a47ae4；安装恢复已由P13-jx完成，沿最新P13-kn仍待。设计读取为原生紧凑配置浮层，冒险3、动效2、密度7；用户三秒内知道仍在改哪一项，并能继续原操作。

## 动手前证据与八项候选

`internal/settingsui/pluginsheet.go:532–546`只在FocusWithin由false变true时显现；`:205`的失败行及`:238`的分组会改变位置而不改变焦点。先真实事件复现，不把代码线索算完成。

| 项 | 位置（开盘） | 现状 → 目标 |
| --- | --- | --- |
| 1 Radio | pluginsheet.go:468 | 选项/组更新后原轨道不重显现 → 原焦点与完整活动墨 |
| 2 Checkboxes | pluginsheet.go:499 | 选项重排/换行后焦点仍在旧视口外 → 当前选项显现 |
| 3 Text | pluginsheet.go:738 | 失败行插入后编辑槽移出视口 → 留住未提交草稿并显现 |
| 4 Number | pluginsheet.go:696 | 提示/分组刷新后数字槽不可见 → 半输入保留、外槽显现 |
| 5 Toggle | pluginsheet.go:351 | 刷新后开关环被淡区吞掉 → 当前开关显现 |
| 6 Select | pluginsheet.go:356 | 分组变化后下拉入口离开视口 → 当前入口显现 |
| 7 Slider | pluginsheet.go:563 | 宿主更新布局后轨道消失 → 轨道与原按键写回可用 |
| 8 Action | pluginsheet.go:813 | 提示/分组改变后动作出口离开视口 → 原确认模型与可见按钮 |

目标：只在宿主布局输入实际改变时重显现，沿用16px绝对、指针穿透矩形。值刷新和普通滚轮不能持续抢回焦点。高度链、键盘含义、会话管理、设置写回、i18n、命令接线及3秒窗口不变。

## 验证

开盘完整verbose正在实跑；尚不以旧报告6570/3553代替本轮实测。逐两项定向测试、包vet通过即提交推送；轮末gofmt、三平台vet/build、普通/verbose全量。旧V/B/S断言零改，新增事件pin断言几何、焦点、写回归属、滚轮自由；深浅before/after真实ui.Tester PNG入/tmp及档案。

M1撤销Radio/Checkboxes刷新显现；M2撤销文本/数字刷新显现：注入→原始实红→恢复→md5sum -c→touch→复跑。不得放宽边界或跳测。

## 设计取用

已读qiaomu-design、user-preferences、craft-loop、preflight、chinese-typography及Linear DESIGN。取其三级面、0.05–0.08发丝边、单一交互accent职责，映射本仓既有token。craft-loop #3保留3px焦点环、#4刷新时一次16px/0ms显现、#12增加玻璃面/阴影0层。为何更轻有空间：错误与刷新不会让当前操作掉进浮层软边，继续保持底/面/浮的原层次。现有方向明确，不走四方向预览。
