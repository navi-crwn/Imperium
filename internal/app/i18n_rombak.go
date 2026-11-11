package app

// Labels of the redesigned admin: the Actions menu, the confirmation dialog, the tabs of a user, and one
// line for every role. Nine languages: en, zh-CN, de, id, ja, pt-BR, es, fr, ru.

var rombakKeys = []string{
	"act.menu", "act.confirm", "bulk.confirm_title", "tab.profile", "tab.content", "tab.sessions", "tab.activity",
	"role.owner", "role.admin", "role.moderator", "role.support", "role.app", "dlg.typed", "err.need_ack", "ext.run", "err.need_reason",
}

var rombakVals = map[string][]string{
	"en": {"Actions", "Confirm", "Apply “{action}” to {count} rows?", "Profile", "Content", "Sessions", "Activity",
		"Full control of Imperium and every app.", "Manages the users, content and settings of every app.", "Handles reports and content; cannot change settings.", "Read-only access, to help people.", "Manages only this app: its content, reports and settings.", "Type %s to confirm", "Confirm this action in the dialog first.", "Run an action", "A reason is required for this action."},
	"zh-CN": {"操作", "确认", "对 {count} 行执行“{action}”?", "资料", "内容", "会话", "活动",
		"完全控制 Imperium 和所有应用。", "管理所有应用的用户、内容和设置。", "处理举报和内容,不能更改设置。", "只读权限,用于协助用户。", "只管理这一个应用的内容、举报和设置。", "输入 %s 以确认", "请先在对话框中确认此操作。", "执行操作", "此操作必须填写原因。"},
	"de": {"Aktionen", "Bestätigen", "„{action}“ auf {count} Zeilen anwenden?", "Profil", "Inhalt", "Sitzungen", "Aktivität",
		"Volle Kontrolle über Imperium und jede App.", "Verwaltet Nutzer, Inhalte und Einstellungen aller Apps.", "Bearbeitet Meldungen und Inhalte, keine Einstellungen.", "Nur Lesezugriff, um Menschen zu helfen.", "Verwaltet nur diese App: Inhalte, Meldungen und Einstellungen.", "Zur Bestätigung %s eingeben", "Bestätige diese Aktion zuerst im Dialog.", "Aktion ausführen", "Für diese Aktion ist eine Begründung erforderlich."},
	"id": {"Aksi", "Konfirmasi", "Terapkan “{action}” ke {count} baris?", "Profil", "Konten", "Sesi", "Aktivitas",
		"Kendali penuh atas Imperium dan semua aplikasi.", "Mengelola pengguna, konten, dan pengaturan semua aplikasi.", "Menangani laporan dan konten; tidak bisa mengubah pengaturan.", "Hanya baca, untuk membantu pengguna.", "Hanya mengelola aplikasi ini: konten, laporan, dan pengaturannya.", "Ketik %s untuk konfirmasi", "Konfirmasi aksi ini di dialog dulu.", "Jalankan aksi", "Alasan wajib diisi untuk aksi ini."},
	"ja": {"操作", "確認", "「{action}」を {count} 行に適用しますか?", "プロフィール", "コンテンツ", "セッション", "アクティビティ",
		"Imperium とすべてのアプリを完全に管理できます。", "すべてのアプリのユーザー、コンテンツ、設定を管理します。", "報告とコンテンツを扱います。設定は変更できません。", "閲覧のみ。サポート用です。", "このアプリのコンテンツ、報告、設定だけを管理します。", "確認のため %s を入力", "先にダイアログでこの操作を確認してください。", "操作を実行", "この操作には理由が必要です。"},
	"pt-BR": {"Ações", "Confirmar", "Aplicar “{action}” a {count} linhas?", "Perfil", "Conteúdo", "Sessões", "Atividade",
		"Controle total do Imperium e de todos os apps.", "Gerencia usuários, conteúdo e configurações de todos os apps.", "Trata denúncias e conteúdo; não altera configurações.", "Somente leitura, para ajudar pessoas.", "Gerencia só este app: conteúdo, denúncias e configurações.", "Digite %s para confirmar", "Confirme esta ação primeiro no diálogo.", "Executar uma ação", "Esta ação exige um motivo."},
	"es": {"Acciones", "Confirmar", "¿Aplicar «{action}» a {count} filas?", "Perfil", "Contenido", "Sesiones", "Actividad",
		"Control total de Imperium y de todas las apps.", "Gestiona usuarios, contenido y ajustes de todas las apps.", "Atiende denuncias y contenido; no cambia ajustes.", "Solo lectura, para ayudar a las personas.", "Gestiona solo esta app: contenido, denuncias y ajustes.", "Escribe %s para confirmar", "Confirma primero esta acción en el diálogo.", "Ejecutar una acción", "Esta acción requiere un motivo."},
	"fr": {"Actions", "Confirmer", "Appliquer « {action} » à {count} lignes ?", "Profil", "Contenu", "Sessions", "Activité",
		"Contrôle total d'Imperium et de chaque app.", "Gère les utilisateurs, le contenu et les réglages de chaque app.", "Traite les signalements et le contenu; ne change pas les réglages.", "Lecture seule, pour aider les gens.", "Gère uniquement cette app : contenu, signalements et réglages.", "Saisissez %s pour confirmer", "Confirmez d'abord cette action dans la boîte de dialogue.", "Lancer une action", "Cette action exige un motif."},
	"ru": {"Действия", "Подтвердить", "Применить «{action}» к строкам: {count}?", "Профиль", "Контент", "Сессии", "Активность",
		"Полный контроль над Imperium и всеми приложениями.", "Управляет пользователями, контентом и настройками всех приложений.", "Работает с жалобами и контентом; не меняет настройки.", "Только чтение, чтобы помогать людям.", "Управляет только этим приложением: контентом, жалобами и настройками.", "Введите %s для подтверждения", "Сначала подтвердите действие в диалоге.", "Выполнить действие", "Для этого действия нужна причина."},
}

func init() {
	for lang, vals := range rombakVals {
		if catalog[lang] == nil {
			catalog[lang] = map[string]string{}
		}
		for i, k := range rombakKeys {
			catalog[lang][k] = vals[i]
		}
	}
}
