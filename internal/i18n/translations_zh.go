package i18n

// Chinese translations (Simplified)
var chineseTranslations = map[string]string{
	// General
	"app.name":        "DevOrch",
	"app.description": "AI驱动的开发编排工具",
	"app.version":     "版本 %s",

	// Commands
	"cmd.help":    "显示帮助",
	"cmd.version": "显示版本",
	"cmd.config":  "配置设置",
	"cmd.chat":    "开始聊天会话",
	"cmd.run":     "运行任务",
	"cmd.init":    "初始化项目",

	// Chat
	"chat.welcome":     "欢迎使用DevOrch！输入消息或/help查看命令。",
	"chat.thinking":    "思考中...",
	"chat.typing":      "输入中...",
	"chat.error":       "错误: %s",
	"chat.exit":        "再见！",
	"chat.clear":       "聊天已清除。",
	"chat.saved":       "聊天已保存到 %s",
	"chat.loaded":      "聊天已从 %s 加载",
	"chat.new_session": "正在开始新会话...",

	// Tools
	"tool.executing":  "正在执行 %s...",
	"tool.completed":  "%s 完成",
	"tool.failed":     "失败: %s",
	"tool.permission": "需要 %s 的权限",
	"tool.approve":    "批准",
	"tool.deny":       "拒绝",
	"tool.always":     "始终允许",

	// Files
	"file.reading":  "正在读取 %s...",
	"file.writing":  "正在写入 %s...",
	"file.creating": "正在创建 %s...",
	"file.deleting": "正在删除 %s...",
	"file.modified": "文件已修改: %s",
	"file.created":  "文件已创建: %s",
	"file.deleted":  "文件已删除: %s",
	"file.notfound": "找不到文件: %s",

	// Git
	"git.staging":    "正在暂存更改...",
	"git.committing": "正在提交...",
	"git.pushing":    "正在推送...",
	"git.pulling":    "正在拉取...",
	"git.status":     "Git状态",
	"git.diff":       "Git差异",

	// Providers
	"provider.connecting": "正在连接到 %s...",
	"provider.connected":  "已连接到 %s",
	"provider.error":      "提供商错误: %s",
	"provider.ratelimit":  "已达到速率限制。正在等待...",
	"provider.retry":      "%d秒后重试...",

	// Settings
	"settings.saved":   "设置已保存",
	"settings.loaded":  "设置已加载",
	"settings.reset":   "设置已重置为默认值",
	"settings.invalid": "无效设置: %s",

	// Errors
	"error.generic":     "发生错误: %s",
	"error.network":     "网络错误: %s",
	"error.permission":  "权限被拒绝: %s",
	"error.notfound":    "未找到: %s",
	"error.invalid":     "无效输入: %s",
	"error.timeout":     "操作超时",
	"error.interrupted": "操作被中断",

	// Confirmations
	"confirm.yes":     "是",
	"confirm.no":      "否",
	"confirm.cancel":  "取消",
	"confirm.proceed": "要继续吗？",
	"confirm.delete":  "确定要删除 %s 吗？",
	"confirm.reset":   "确定要重置吗？",

	// Status
	"status.ready":      "就绪",
	"status.busy":       "忙碌",
	"status.waiting":    "等待中...",
	"status.processing": "处理中...",
	"status.complete":   "完成",
	"status.failed":     "失败",

	// TUI
	"tui.input.placeholder": "输入消息...",
	"tui.sidebar.chats":     "聊天",
	"tui.sidebar.tools":     "工具",
	"tui.sidebar.settings":  "设置",
	"tui.help.title":        "键盘快捷键",
	"tui.help.quit":         "退出",
	"tui.help.send":         "发送消息",
	"tui.help.newline":      "换行",
	"tui.help.clear":        "清除聊天",
	"tui.help.copy":         "复制最后响应",

	// Web UI
	"web.title":      "DevOrch网页界面",
	"web.connect":    "连接",
	"web.disconnect": "断开连接",
	"web.settings":   "设置",
	"web.history":    "历史",
	"web.new_chat":   "新聊天",
	"web.export":     "导出",
	"web.import":     "导入",

	// Memory
	"memory.title":    "项目内存",
	"memory.saved":    "内存已保存",
	"memory.loaded":   "内存已加载",
	"memory.cleared":  "内存已清除",
	"memory.notfound": "未找到内存",

	// Session
	"session.new":       "新会话已创建",
	"session.restored":  "会话已恢复",
	"session.compacted": "会话已压缩: %d条消息",
	"session.expired":   "会话已过期",
}
