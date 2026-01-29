package i18n

// Japanese translations
var japaneseTranslations = map[string]string{
	// General
	"app.name":        "DevOrch",
	"app.description": "AI駆動の開発オーケストレーションツール",
	"app.version":     "バージョン %s",

	// Commands
	"cmd.help":    "ヘルプを表示",
	"cmd.version": "バージョンを表示",
	"cmd.config":  "設定を構成",
	"cmd.chat":    "チャットセッションを開始",
	"cmd.run":     "タスクを実行",
	"cmd.init":    "プロジェクトを初期化",

	// Chat
	"chat.welcome":     "DevOrchへようこそ！メッセージを入力するか、/helpでコマンドを確認してください。",
	"chat.thinking":    "考え中...",
	"chat.typing":      "入力中...",
	"chat.error":       "エラー: %s",
	"chat.exit":        "さようなら！",
	"chat.clear":       "チャットがクリアされました。",
	"chat.saved":       "チャットが%sに保存されました",
	"chat.loaded":      "チャットが%sからロードされました",
	"chat.new_session": "新しいセッションを開始しています...",

	// Tools
	"tool.executing":  "%sを実行中...",
	"tool.completed":  "%sが完了しました",
	"tool.failed":     "失敗: %s",
	"tool.permission": "%sの権限が必要です",
	"tool.approve":    "承認",
	"tool.deny":       "拒否",
	"tool.always":     "常に許可",

	// Files
	"file.reading":  "%sを読み込み中...",
	"file.writing":  "%sに書き込み中...",
	"file.creating": "%sを作成中...",
	"file.deleting": "%sを削除中...",
	"file.modified": "ファイルが変更されました: %s",
	"file.created":  "ファイルが作成されました: %s",
	"file.deleted":  "ファイルが削除されました: %s",
	"file.notfound": "ファイルが見つかりません: %s",

	// Git
	"git.staging":    "変更をステージング中...",
	"git.committing": "コミット中...",
	"git.pushing":    "プッシュ中...",
	"git.pulling":    "プル中...",
	"git.status":     "Gitステータス",
	"git.diff":       "Git差分",

	// Providers
	"provider.connecting": "%sに接続中...",
	"provider.connected":  "%sに接続しました",
	"provider.error":      "プロバイダーエラー: %s",
	"provider.ratelimit":  "レート制限に達しました。待機中...",
	"provider.retry":      "%d秒後にリトライ...",

	// Settings
	"settings.saved":   "設定が保存されました",
	"settings.loaded":  "設定がロードされました",
	"settings.reset":   "設定がデフォルトにリセットされました",
	"settings.invalid": "無効な設定: %s",

	// Errors
	"error.generic":     "エラーが発生しました: %s",
	"error.network":     "ネットワークエラー: %s",
	"error.permission":  "権限が拒否されました: %s",
	"error.notfound":    "見つかりません: %s",
	"error.invalid":     "無効な入力: %s",
	"error.timeout":     "操作がタイムアウトしました",
	"error.interrupted": "操作が中断されました",

	// Confirmations
	"confirm.yes":     "はい",
	"confirm.no":      "いいえ",
	"confirm.cancel":  "キャンセル",
	"confirm.proceed": "続行しますか？",
	"confirm.delete":  "%sを削除しますか？",
	"confirm.reset":   "リセットしますか？",

	// Status
	"status.ready":      "準備完了",
	"status.busy":       "処理中",
	"status.waiting":    "待機中...",
	"status.processing": "処理中...",
	"status.complete":   "完了",
	"status.failed":     "失敗",

	// TUI
	"tui.input.placeholder": "メッセージを入力...",
	"tui.sidebar.chats":     "チャット",
	"tui.sidebar.tools":     "ツール",
	"tui.sidebar.settings":  "設定",
	"tui.help.title":        "キーボードショートカット",
	"tui.help.quit":         "終了",
	"tui.help.send":         "メッセージを送信",
	"tui.help.newline":      "改行",
	"tui.help.clear":        "チャットをクリア",
	"tui.help.copy":         "最後の応答をコピー",

	// Web UI
	"web.title":      "DevOrch Web UI",
	"web.connect":    "接続",
	"web.disconnect": "切断",
	"web.settings":   "設定",
	"web.history":    "履歴",
	"web.new_chat":   "新しいチャット",
	"web.export":     "エクスポート",
	"web.import":     "インポート",

	// Memory
	"memory.title":    "プロジェクトメモリ",
	"memory.saved":    "メモリが保存されました",
	"memory.loaded":   "メモリがロードされました",
	"memory.cleared":  "メモリがクリアされました",
	"memory.notfound": "メモリが見つかりません",

	// Session
	"session.new":       "新しいセッションが作成されました",
	"session.restored":  "セッションが復元されました",
	"session.compacted": "セッションが圧縮されました: %d件のメッセージ",
	"session.expired":   "セッションが期限切れです",
}
