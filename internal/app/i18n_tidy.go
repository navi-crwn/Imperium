package app

// Labels for the tidied dashboard: the "More" tab menu, the names of extension pages,
// paging captions, and readable names for keys the apps report (api_tokens, clicks_30d,
// last_activity, …) that used to be printed raw. Nine languages: en, zh-CN, de, id, ja,
// pt-BR, es, fr, ru. A key an app reports that is not listed here is spelled out in
// plain words by keyLabel instead of being shown with underscores.

var tidyKeys = []string{
	"nav.more", "ext.analytics", "ext.guest_digest", "raw.json", "page.showing", "page.show_more",
	"lbl.api_tokens", "lbl.clicks_30d", "lbl.last_activity", "lbl.count", "lbl.links", "lbl.bio_pages",
	"lbl.holds", "lbl.reserved", "lbl.bans", "lbl.watchlist", "lbl.domain", "lbl.slug", "lbl.reason",
	"lbl.until", "lbl.ip", "lbl.created_at", "lbl.expires_at", "lbl.category", "lbl.attempts", "lbl.user",
	"lbl.match_type", "lbl.notes", "lbl.id", "col.account",
}

var tidyVals = map[string][]string{
	"en": {"More", "Analytics", "Guest link digest", "Raw data (JSON)", "Showing %d of %d", "Show more",
		"API tokens", "Clicks (30 days)", "Last activity", "Total", "Links", "Bio pages",
		"Holds", "Reserved", "Bans", "Watchlist", "Domain", "Slug", "Reason",
		"Until", "IP", "Created", "Expires", "Category", "Attempts", "User",
		"Match", "Notes", "ID", "Account"},
	"zh-CN": {"更多", "分析", "访客链接摘要", "原始数据(JSON)", "显示 %d / %d", "显示更多",
		"API 令牌", "点击(30 天)", "最近活动", "总数", "链接", "Bio 页面",
		"保留项", "保留名", "封禁", "观察名单", "域名", "短名", "原因",
		"截止", "IP", "创建时间", "过期时间", "类别", "尝试次数", "用户",
		"匹配方式", "备注", "ID", "账户"},
	"de": {"Mehr", "Analytik", "Gästelink-Übersicht", "Rohdaten (JSON)", "%d von %d angezeigt", "Mehr anzeigen",
		"API-Token", "Klicks (30 Tage)", "Letzte Aktivität", "Gesamt", "Links", "Bio-Seiten",
		"Sperrfristen", "Reserviert", "Sperren", "Beobachtungsliste", "Domain", "Slug", "Grund",
		"Bis", "IP", "Erstellt", "Läuft ab", "Kategorie", "Versuche", "Benutzer",
		"Abgleich", "Notizen", "ID", "Konto"},
	"id": {"Lainnya", "Analitik", "Ringkasan tautan tamu", "Data mentah (JSON)", "Menampilkan %d dari %d", "Tampilkan lebih banyak",
		"Token API", "Klik (30 hari)", "Aktivitas terakhir", "Total", "Tautan", "Halaman bio",
		"Penahanan", "Dicadangkan", "Blokir", "Daftar pantauan", "Domain", "Slug", "Alasan",
		"Sampai", "IP", "Dibuat", "Kedaluwarsa", "Kategori", "Percobaan", "Pengguna",
		"Pencocokan", "Catatan", "ID", "Akun"},
	"ja": {"その他", "分析", "ゲストリンクの概要", "生データ(JSON)", "%d / %d 件を表示", "さらに表示",
		"API トークン", "クリック(30日)", "最終アクティビティ", "合計", "リンク", "Bio ページ",
		"保留", "予約済み", "ブロック", "監視リスト", "ドメイン", "スラッグ", "理由",
		"期限", "IP", "作成日", "有効期限", "カテゴリ", "試行回数", "ユーザー",
		"一致方式", "メモ", "ID", "アカウント"},
	"pt-BR": {"Mais", "Análises", "Resumo de links de convidados", "Dados brutos (JSON)", "Mostrando %d de %d", "Mostrar mais",
		"Tokens de API", "Cliques (30 dias)", "Última atividade", "Total", "Links", "Páginas bio",
		"Retenções", "Reservados", "Bloqueios", "Lista de observação", "Domínio", "Slug", "Motivo",
		"Até", "IP", "Criado", "Expira", "Categoria", "Tentativas", "Usuário",
		"Correspondência", "Notas", "ID", "Conta"},
	"es": {"Más", "Analítica", "Resumen de enlaces de invitados", "Datos sin procesar (JSON)", "Mostrando %d de %d", "Mostrar más",
		"Tokens de API", "Clics (30 días)", "Última actividad", "Total", "Enlaces", "Páginas bio",
		"Retenciones", "Reservados", "Bloqueos", "Lista de vigilancia", "Dominio", "Slug", "Motivo",
		"Hasta", "IP", "Creado", "Caduca", "Categoría", "Intentos", "Usuario",
		"Coincidencia", "Notas", "ID", "Cuenta"},
	"fr": {"Plus", "Statistiques", "Résumé des liens invités", "Données brutes (JSON)", "%d sur %d affichés", "Afficher plus",
		"Jetons API", "Clics (30 jours)", "Dernière activité", "Total", "Liens", "Pages bio",
		"Retenues", "Réservés", "Blocages", "Liste de surveillance", "Domaine", "Slug", "Motif",
		"Jusqu'au", "IP", "Créé", "Expire", "Catégorie", "Tentatives", "Utilisateur",
		"Correspondance", "Notes", "ID", "Compte"},
	"ru": {"Ещё", "Аналитика", "Сводка гостевых ссылок", "Сырые данные (JSON)", "Показано %d из %d", "Показать ещё",
		"API-токены", "Клики (30 дней)", "Последняя активность", "Всего", "Ссылки", "Bio-страницы",
		"Удержания", "Зарезервировано", "Блокировки", "Список наблюдения", "Домен", "Слаг", "Причина",
		"До", "IP", "Создано", "Истекает", "Категория", "Попытки", "Пользователь",
		"Совпадение", "Заметки", "ID", "Аккаунт"},
}

func init() {
	for lang, vals := range tidyVals {
		if catalog[lang] == nil {
			catalog[lang] = map[string]string{}
		}
		for i, k := range tidyKeys {
			catalog[lang][k] = vals[i]
		}
	}
}
