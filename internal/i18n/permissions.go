package i18n

// PermissionLabel names a permission id for the user, as the old build's
// settings.extensions.permission.* keys did.
func PermissionLabel(language, permission string) string {
	table := permissionLabelsEN
	if Normalize(language) == ZH {
		table = permissionLabelsZH
	}
	if label, ok := table[permission]; ok {
		return label
	}
	return permission
}

var permissionLabelsEN = map[string]string{
	"filesystem-read":  "Read files",
	"filesystem-write": "Modify files",
	"network-fetch":    "Access the network",
	"process-spawn":    "Start processes",
	"clipboard-read":   "Read the clipboard",
	"clipboard-write":  "Write the clipboard",
	"environment":      "Read the environment",
}

var permissionLabelsZH = map[string]string{
	"filesystem-read":  "读取文件",
	"filesystem-write": "修改文件",
	"network-fetch":    "访问网络",
	"process-spawn":    "启动进程",
	"clipboard-read":   "读取剪贴板",
	"clipboard-write":  "写入剪贴板",
	"environment":      "读取环境变量",
}
