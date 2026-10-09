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

// PermissionDescription explains what a permission allows, for the audit list
// the Integrations page opens: the label names it, this says what it means and
// whether the host itself decides it.
func PermissionDescription(language, permission string) string {
	table := permissionDescriptionsEN
	if Normalize(language) == ZH {
		table = permissionDescriptionsZH
	}
	if description, ok := table[permission]; ok {
		return description
	}
	return ""
}

var permissionDescriptionsEN = map[string]string{
	"filesystem-read":  "The tool may read files your user can read.",
	"filesystem-write": "The tool may modify or create files your user can write.",
	"network-fetch":    "The tool may reach the network.",
	"process-spawn":    "The tool may start other programs. Floter enforces this one: a command runs only the program its manifest declares.",
	"clipboard-read":   "The tool may read what is on your clipboard.",
	"clipboard-write":  "The tool may replace what is on your clipboard.",
	"environment":      "The tool may read your environment variables. Floter enforces this one: the host decides what the tool receives.",
}

var permissionDescriptionsZH = map[string]string{
	"filesystem-read":  "该工具可以读取你的用户可读的文件。",
	"filesystem-write": "该工具可以修改或创建你的用户可写的文件。",
	"network-fetch":    "该工具可以访问网络。",
	"process-spawn":    "该工具可以启动其它程序。这一条由 floter 强制：只会运行清单声明的那个程序。",
	"clipboard-read":   "该工具可以读取你剪贴板里的内容。",
	"clipboard-write":  "该工具可以改写你剪贴板里的内容。",
	"environment":      "该工具可以读取你的环境变量。这一条由 floter 强制：宿主决定它拿到什么。",
}
