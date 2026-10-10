package pinyin

import "strings"

// The pinyin first-letter table: the ~2200 characters of the GB2312 level-1
// set, grouped by their most common reading's initial letter. It answers the
// same question the old build's `pinyin::ToPinyin` did, restricted to the
// characters application names actually use — a name with a character outside
// the table keeps the rest of its initials, which is what makes partial
// coverage workable.
//
// A multi-reading character (重, 长, 了, …) is listed under one reading only:
// the launcher matches initials as a subsequence over a name, so a character
// read two ways is reachable through the other characters, and claiming both
// would muddy every name that shares them.

var initialsByReading = []struct {
	letter string
	chars  string
}{
	{"w", "万丸为乌五亡卫危伟伪位务味胃喂慰温文纹闻稳窝我卧握无吴武误悟雾汪王往忘旺望威微为围违唯委未网"},
	{"b", "丙伴便保倍八兵冰别剥办勃包北半博变吧奔宝宾币布帮并庇弊彼必悲扁把报拔拜拨捕搏播暴本杯板柄柏标棒步比毕波滨爸版玻班璧病白百笔笨簿绑编罢背膊臂般薄补表被豹贝败跋辈辨辩边遍避部鄙闭靶鞭饱饼驳"},
	{"p", "仆佩偏僻凭判剖匹叛品啪坡埔培婆屁帕平怕扒批抛拍拼捧排攀普朋棚泼派漂瀑炮爬片牌瓶疲皮盆盘破票篇聘膨苹菩蓬譬评谱赔迫配陪飘骗魄频"},
	{"m", "么亩们免勉卖名吗命墓墨妈妹姆密帽幕庙忙慕慢抹描摩摸敏明暮曼木末某梦棉模母每毛民没沫满漠漫灭牧猛猫目眠眯瞒矛码磨秒秘米绵美脉膜芒苗茂莽蒙蚂蜜谋谜贸迈迷门闷面马骂鸣麦麻默"},
	{"f", "丰付份仿伏佛俯傅凡分副匪反发否啡坊坟复夫奋妇妨孵富封峰帆幅废弗愤房扶抚拂放敷服沸法泛浮烦父犯番疯福符粉纷纺缝罚翻肤肥腐腹芬范蜂蝠覆访负贩费赋赴辅辐返逢锋防附非风飞饭"},
	{"d", "丁东丢丹代但低倒党典冬冻凳刀刁到动单叠叨叮叼吊吨呆地垫堆堕堤堵多大夺定对导岛帝带底店度弟弹当待得德怠悼懂戴打抖抵担挡搭敌斗断旦朵杜栋档段殿毒洞淡渡滴灯点爹独电登的盗盯盾督短稻端笛第等答缎肚胆舵荡董蛋蝶袋订诞读调豆赌跌蹈蹲躲达迪递逗逮道都钉钓镀队陡雕顶顿颠鼎"},
	{"t", "亭他伍体停偷兔午厅台叹同吐吞听唐团图土坛坦堂塌塔塘填外天太头套她妥娃它完尾屋帖庭廷弯徒态托投抬拖挑挖挺挽掏探推提摊晚替条桃桐梯椭歪污汤泰涂涛淘添湾滔滩潭炭烫特玩瓦甜田疼痛痰碗突筒糖纬统维翁脱腿舞袜讨谈贪贴趟跳踏踢躺退逃透途通铁铜陶题驼腾"},
	{"n", "乃乒乓你偶内农凝努南呕呢哦哪女奴奶娘嫩宁尿年弄念恼您扭拿挪捏暖欧泥浓牛男纳纽耐能脑诺趴逆那闹难鸟"},
	{"l", "两临丽乱亮令伦例俐兰冷凉凌凛列刘利劣励劳勒卢历厉厘另吝啦垄垒屡岭峦帘廉廊录律怜懒抡拉拦捞掠搂料旅朗李来林柳栏栗梁梨楼泪洛流浏浪涝涟淋溜漏漓灵炉炼烂烈牢犁狼猎率玲琳璃留略疗碌礼离立笼箩篮类粒粮累练络绿罗老聋联腊良芦荔莉莲萝落蓝虑蜡螺裂裸览论赁赖路轮辆辣辽连邻郎里量铃锣陆陵隆隶零雷露领驴骆骡鲁鳞鹿龙理"},
	{"g", "个乖估供光公共关冠刚刮割功勾古各告哥固国够姑孤官宫岗工干广弓归怪恭感拐拱挂搞改攻故敢更杆杠构果根格桂棍歌沟港滚灌瓜甘盖硅稿糕纲给缸罐耕肝股胳裹观规该谷贡购贯贵赶跟轨辜过钢锅阁隔革顾馆骨高鬼鸽鼓龟管"},
	{"k", "亏克况凯刊刻勘口可咳哭困坎坑块垦垮堪壳夸孔客宽寇库康开快恐恳愧慨慷扛扣扩抗括挎捆控昆枯框棵款渴炕烤狂看矿砍科空筐考肯苦葵裤课跨酷阔靠颗"},
	{"h", "乎互伙会何候划化华厚号合后含吼呼和哄哈哗唤喉喊喝回坏壶好婚孩害寒很忽怀恒恢恨悍悔患惑惠慌慧或户护挥捍换旱昏晃核桓横欢毁毫汇汉汗河洪活浑浩海淮混湖滑火灰焊煌狐狠猴猾环痕皇盒祸禾糊红绘缓胡航花荒荷获虎虹衡话谎豪货贺贿轰辉还霍骸魂黄黑"},
	{"j", "举久九井交京仅今介件价佳俊俱借倦假健僵具军决净几击剂剑剧加匠即卷及句叫吉嘉均坚基境夹奖姐姜娇嫁季家寄将尖就尽局居届峻崛巨己建忌急悸惊惧戒拒捐掘接揭搅救教敬既旧晋景晶机极架检椒歼江洁津浆济渐激炯焦甲界疆疾皆监矩禁积竞竟竭筋简箭籍精紧级纪经结绝继绩缴聚肌肩胶脚舅舰艰节茎菊菌见觉角解警计记讲谨距践轿较近进郊酒酱金鉴锦键锯镜间阶际降集静饥饺驾骄鸡剪"},
	{"q", "七且丘乔乞亲企侨侵倾全其切前劝勤区千却去取启器圈奇契妻娶屈岂巧庆弃强怯恰悄情戚抢拳敲旗晴曲期权枪栖桥棋欠欺气求汽泉洽浅清漆潜牵犬球琴瞧砌确秋穷窃签缺群腔蜻请谦趋趣轻迁遣钱铅锹雀青顷驱骑齐"},
	{"x", "下习乡仙休信修像兄先兴写凶刑协叙向吓吸咸响喜型夏夕姓媳孝学宣宪宵寻小峡巡希席幸序弦形徐循心息悉悬惜想戏掀效斜新旋星显晓晰朽析械欣洗消溪熄熊牺狭献现相硝秀稀穴箱系絮纤线细绣绪续羞羡肖胁胸腥膝萧薪虚虾血行袖袭襄西训许询详谐谢象贤辛迅选醒销锡闲限险陷隙雄雪需霞项须馅香鲜讯"},
	{"r", "乳人仁仍任儒入刃嚷壤如容弱忍惹扔扰揉日染柔润溶热然燃瑞绒绕肉若荣融认让软辱锐"},
	{"z", "丈中主之争仗众住作侄侦债做兆再准则制助匝占只周咱哲啄嘱嘴在坐增壮奏姿子字宅宗宰寨展崭州左帐座张征志忠怎总憎战扎执找折招择指挣振掌掷揍摘撞攒支政整斩族早昨昼智暂最朱杂枕枝枣株栽桌棕榨正汁治沼沾注泽洲浊浙涨渣滋澡灶灾炸烛照煮燥状猪珍珠皱盏直真眨着睁瞻知砸种租秩窄站章粘粥糟紫纵组织终综罩罪置者肇肘脂脏自致舟芝葬蒸蜘装证诊诸贞账质贮贼赃资赞赠走赵足趾踪轴载这追逐遭遮郑针钟钻镇闸阵阻障震驻骤"},
	{"s", "三上世丝丧书事什伞伤伸似使侍俗傻僧兽删刷剩势勺升双叔受史司商善嗓四圣士声失始婶嫂孙守实审宿寺寿少尚尸属山岁市师思慎所扇手扫拾损授搜摔撒撕收散数斯施时晒术杀束松柿树桑梳森死殊氏水汕沈沙洒涉深湿熟狮甚生甩申疏瘦盛省瞬石硕示社神私税竖笋筛算素索纱绍绳缩耍耸肃肾舍舒艘色苏萨蔬薯虽蚀蛇衰誓设识诉试诗说诵赏赛身输述送逝速锁闪陕随顺颂食饲首驶鼠视"},
	{"c", "丑丛串乘产仇从仓传侧促倡偿催充册冲凑出创初刺匙厂厨参叉吃吵吹唱喘嘲场垂城处存察寸尘尝尺层岔崇川差常床彩彻惩慈成戳才扯承抄抽持撑操敞春朝材村查柴楚次此残池沉测潮灿瓷畅疮瞅磁秤称穿窗窜策粗纯缠翅耻肠脆臣臭舱船苍茶草菜藏虫蚕蠢裁触词诚财趁超踩车辞迟采钞锄错锤长闯阐陈除颤餐齿重"},
	{"a", "傲哀唉啊奥安岸按挨暗案氨澳爱癌皑矮碍阿"},
	{"e", "二俄儿尔恩恶而耳蛾额饵饿鹅"},
	{"y", "一与业严义乙也于亚亦亿以仪仰优余依允元养冤冶勇匀医印厌原又友右叶咬因园圆域夜央妖姨姻娱孕宇宜宴尤已幼应庸异引影役御忆怨悠悦愈愉意愿扬拥掩援摇易映晕月有杨样椅榆樱欲殃毅氧永油泳浴涌液渔游源演焰燕爷狱玉用由疑疫痒盈盐眼研秧移约缘羊羽耀育腰艳艺英药营蚁蝇衣要誉议译语诱赢跃迎运远逾遇遗遥邀邮野银阅阳阴院隐雁雨音韵页预颜饮验鱼鹰云乐"},
}

// Initial returns the pinyin first letter of a CJK character, empty when the
// table does not know it.
func Initial(char rune) string {
	for _, group := range initialsByReading {
		if strings.ContainsRune(group.chars, char) {
			return group.letter
		}
	}
	return ""
}

// Initials is the pinyin first-letter key of a name: every ASCII letter and
// digit kept as it is, every CJK character replaced by its reading's first
// letter, and everything else dropped — separators contribute nothing, so a
// query typed as one word still matches across them. This is the shape
// `compute_initials` produced for an application's name and its localization
// folded together.
func Initials(name string) string {
	var builder strings.Builder
	for _, char := range name {
		switch {
		case char >= 'A' && char <= 'Z':
			builder.WriteRune(char + 32)
		case (char >= 'a' && char <= 'z') || (char >= '0' && char <= '9'):
			builder.WriteRune(char)
		default:
			if letter := Initial(char); letter != "" {
				builder.WriteString(letter)
			}
		}
	}
	return builder.String()
}
