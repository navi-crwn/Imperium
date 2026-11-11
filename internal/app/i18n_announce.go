package app

// Labels of the announcements pages and of the role and self-protection messages of the user page. One line
// per key, in the order of langOrder: en, zh-CN, de, id, ja, pt-BR, es, fr, ru.
var announceText = map[string][9]string{
	"nav.announcements":   {"Announcements", "公告", "Ankündigungen", "Pengumuman", "お知らせ", "Avisos", "Anuncios", "Annonces", "Объявления"},
	"ann.title":           {"Announcements", "公告", "Ankündigungen", "Pengumuman", "お知らせ", "Avisos", "Anuncios", "Annonces", "Объявления"},
	"ann.new":             {"New announcement", "新建公告", "Neue Ankündigung", "Pengumuman baru", "お知らせを作成", "Novo aviso", "Nuevo anuncio", "Nouvelle annonce", "Новое объявление"},
	"ann.edit":            {"Edit", "编辑", "Bearbeiten", "Ubah", "編集", "Editar", "Editar", "Modifier", "Изменить"},
	"ann.none":            {"No announcements.", "暂无公告。", "Keine Ankündigungen.", "Belum ada pengumuman.", "お知らせはありません。", "Nenhum aviso.", "No hay anuncios.", "Aucune annonce.", "Объявлений нет."},
	"ann.save":            {"Save", "保存", "Speichern", "Simpan", "保存", "Salvar", "Guardar", "Enregistrer", "Сохранить"},
	"ann.created":         {"Announcement created.", "公告已创建。", "Ankündigung erstellt.", "Pengumuman dibuat.", "お知らせを作成しました。", "Aviso criado.", "Anuncio creado.", "Annonce créée.", "Объявление создано."},
	"ann.updated":         {"Announcement updated.", "公告已更新。", "Ankündigung aktualisiert.", "Pengumuman diperbarui.", "お知らせを更新しました。", "Aviso atualizado.", "Anuncio actualizado.", "Annonce mise à jour.", "Объявление обновлено."},
	"ann.author":          {"Author", "作者", "Autor", "Penulis", "作成者", "Autor", "Autor", "Auteur", "Автор"},
	"ann.live_hint":       {"Published notices show on hel.ink, in the launcher and on /announcements.", "已发布的公告会显示在 hel.ink、启动器和 /announcements。", "Veröffentlichte Hinweise erscheinen auf hel.ink, im Launcher und unter /announcements.", "Pengumuman yang terbit tampil di hel.ink, peluncur, dan /announcements.", "公開したお知らせは hel.ink、ランチャー、/announcements に表示されます。", "Os avisos publicados aparecem em hel.ink, no lançador e em /announcements.", "Los anuncios publicados aparecen en hel.ink, en el lanzador y en /announcements.", "Les annonces publiées s'affichent sur hel.ink, dans le lanceur et sur /announcements.", "Опубликованные объявления показываются на hel.ink, в лаунчере и на /announcements."},
	"ann.app_all":         {"HOP (all apps)", "HOP(所有应用)", "HOP (alle Apps)", "HOP (semua aplikasi)", "HOP(すべてのアプリ)", "HOP (todos os apps)", "HOP (todas las apps)", "HOP (toutes les apps)", "HOP (все приложения)"},
	"ann.state.draft":     {"Draft", "草稿", "Entwurf", "Draf", "下書き", "Rascunho", "Borrador", "Brouillon", "Черновик"},
	"ann.state.scheduled": {"Scheduled", "已排期", "Geplant", "Terjadwal", "予約済み", "Agendado", "Programado", "Planifiée", "Запланировано"},
	"ann.state.live":      {"Live", "已上线", "Aktiv", "Tayang", "公開中", "No ar", "Publicado", "En ligne", "Опубликовано"},
	"ann.state.expired":   {"Expired", "已过期", "Abgelaufen", "Kedaluwarsa", "期限切れ", "Expirado", "Caducado", "Expirée", "Истекло"},
	"ann.level.info":      {"Info", "信息", "Info", "Info", "情報", "Informação", "Información", "Info", "Инфо"},
	"ann.level.warning":   {"Warning", "警告", "Warnung", "Peringatan", "警告", "Aviso", "Advertencia", "Avertissement", "Предупреждение"},
	"ann.level.critical":  {"Critical", "严重", "Kritisch", "Kritis", "重大", "Crítico", "Crítico", "Critique", "Критично"},
	"ann.col.title":       {"Title", "标题", "Titel", "Judul", "タイトル", "Título", "Título", "Titre", "Заголовок"},
	"ann.col.state":       {"State", "状态", "Status", "Status", "状態", "Estado", "Estado", "État", "Состояние"},
	"ann.col.level":       {"Level", "级别", "Stufe", "Tingkat", "レベル", "Nível", "Nivel", "Niveau", "Уровень"},
	"ann.col.app":         {"App", "应用", "App", "Aplikasi", "アプリ", "App", "App", "App", "Приложение"},
	"ann.col.published":   {"Published", "发布时间", "Veröffentlicht", "Terbit", "公開日時", "Publicado em", "Publicado", "Publiée le", "Опубликовано"},
	"ann.col.expires":     {"Expires", "到期时间", "Läuft ab", "Kedaluwarsa", "有効期限", "Expira em", "Caduca", "Expire le", "Истекает"},
	"ann.f.title":         {"Title", "标题", "Titel", "Judul", "タイトル", "Título", "Título", "Titre", "Заголовок"},
	"ann.f.body":          {"Text", "正文", "Text", "Isi", "本文", "Texto", "Texto", "Texte", "Текст"},
	"ann.f.level":         {"Level", "级别", "Stufe", "Tingkat", "レベル", "Nível", "Nivel", "Niveau", "Уровень"},
	"ann.f.app":           {"About", "涉及对象", "Betrifft", "Mengenai", "対象", "Sobre", "Sobre", "Concerne", "Относится к"},
	"ann.f.published":     {"Publish at (empty: draft)", "发布时间(留空:草稿)", "Veröffentlichen am (leer: Entwurf)", "Terbit pada (kosong: draf)", "公開日時(空欄:下書き)", "Publicar em (vazio: rascunho)", "Publicar el (vacío: borrador)", "Publier le (vide : brouillon)", "Опубликовать (пусто: черновик)"},
	"ann.f.expires":       {"Expires at (optional)", "到期时间(可选)", "Läuft ab am (optional)", "Kedaluwarsa pada (opsional)", "有効期限(任意)", "Expira em (opcional)", "Caduca el (opcional)", "Expire le (facultatif)", "Истекает (необязательно)"},
	"ann.f.publish_now":   {"Publish now", "立即发布", "Jetzt veröffentlichen", "Terbitkan sekarang", "今すぐ公開", "Publicar agora", "Publicar ahora", "Publier maintenant", "Опубликовать сейчас"},
	"ann.f.utc_hint":      {"Times are in UTC.", "时间为 UTC。", "Zeiten in UTC.", "Waktu dalam UTC.", "時刻は UTC です。", "Os horários estão em UTC.", "Las horas están en UTC.", "Les heures sont en UTC.", "Время указано в UTC."},
	"ann.err.summary":     {"Nothing was saved. Fix the fields marked below.", "未保存。请修正下方标出的字段。", "Nichts wurde gespeichert. Bitte die markierten Felder korrigieren.", "Belum disimpan. Perbaiki kolom yang ditandai di bawah.", "保存されていません。下の印のある項目を修正してください。", "Nada foi salvo. Corrija os campos marcados abaixo.", "No se guardó nada. Corrige los campos marcados abajo.", "Rien n'a été enregistré. Corrigez les champs signalés ci-dessous.", "Ничего не сохранено. Исправьте отмеченные поля."},
	"ann.err.title":       {"Enter a title.", "请输入标题。", "Bitte einen Titel eingeben.", "Isi judul.", "タイトルを入力してください。", "Informe um título.", "Escribe un título.", "Saisissez un titre.", "Введите заголовок."},
	"ann.err.title_long":  {"The title may have at most %d characters.", "标题最多 %d 个字符。", "Der Titel darf höchstens %d Zeichen haben.", "Judul maksimal %d karakter.", "タイトルは %d 文字までです。", "O título pode ter no máximo %d caracteres.", "El título puede tener como máximo %d caracteres.", "Le titre peut compter au plus %d caractères.", "Заголовок — не более %d символов."},
	"ann.err.body":        {"Enter the text of the notice.", "请输入公告正文。", "Bitte den Text eingeben.", "Isi teks pengumuman.", "本文を入力してください。", "Informe o texto do aviso.", "Escribe el texto del anuncio.", "Saisissez le texte de l'annonce.", "Введите текст объявления."},
	"ann.err.body_long":   {"The text may have at most %d characters.", "正文最多 %d 个字符。", "Der Text darf höchstens %d Zeichen haben.", "Teks maksimal %d karakter.", "本文は %d 文字までです。", "O texto pode ter no máximo %d caracteres.", "El texto puede tener como máximo %d caracteres.", "Le texte peut compter au plus %d caractères.", "Текст — не более %d символов."},
	"ann.err.level":       {"Pick info, warning or critical.", "请选择信息、警告或严重。", "Info, Warnung oder Kritisch wählen.", "Pilih info, peringatan, atau kritis.", "情報・警告・重大から選んでください。", "Escolha informação, aviso ou crítico.", "Elige información, advertencia o crítico.", "Choisissez info, avertissement ou critique.", "Выберите инфо, предупреждение или критично."},
	"ann.err.app":         {"Pick HOP or one of the registered apps.", "请选择 HOP 或已注册的应用。", "HOP oder eine registrierte App wählen.", "Pilih HOP atau salah satu aplikasi terdaftar.", "HOP または登録済みのアプリを選んでください。", "Escolha HOP ou um app registrado.", "Elige HOP o una app registrada.", "Choisissez HOP ou une app enregistrée.", "Выберите HOP или зарегистрированное приложение."},
	"ann.err.date":        {"Enter a valid date and time.", "请输入有效的日期和时间。", "Gültiges Datum und Uhrzeit eingeben.", "Isi tanggal dan jam yang valid.", "有効な日時を入力してください。", "Informe uma data e hora válidas.", "Introduce una fecha y hora válidas.", "Saisissez une date et une heure valides.", "Введите корректные дату и время."},
	"ann.err.order":       {"The expiry must be after the publish time.", "到期时间必须晚于发布时间。", "Das Ablaufdatum muss nach der Veröffentlichung liegen.", "Kedaluwarsa harus setelah waktu terbit.", "有効期限は公開日時より後にしてください。", "A expiração deve ser depois da publicação.", "La caducidad debe ser posterior a la publicación.", "L'expiration doit suivre la publication.", "Срок окончания должен быть позже публикации."},
	"ann.err.reason_long": {"The reason may have at most %d characters.", "原因最多 %d 个字符。", "Der Grund darf höchstens %d Zeichen haben.", "Alasan maksimal %d karakter.", "理由は %d 文字までです。", "O motivo pode ter no máximo %d caracteres.", "El motivo puede tener como máximo %d caracteres.", "Le motif peut compter au plus %d caractères.", "Причина — не более %d символов."},
	"err.self":            {"You cannot do this to your own account.", "不能对自己的账号执行此操作。", "Das geht nicht mit dem eigenen Konto.", "Tindakan ini tidak boleh untuk akun Anda sendiri.", "自分のアカウントには実行できません。", "Você não pode fazer isso com a sua própria conta.", "No puedes hacer esto con tu propia cuenta.", "Impossible sur votre propre compte.", "Нельзя сделать это со своим аккаунтом."},
	"err.owner_grant":     {"Ownership is set on HOP ID (SUPERADMIN_EMAIL), not here.", "所有者身份在 HOP ID 中设置(SUPERADMIN_EMAIL),不在此处。", "Eigentümer werden in HOP ID festgelegt (SUPERADMIN_EMAIL), nicht hier.", "Kepemilikan diatur di HOP ID (SUPERADMIN_EMAIL), bukan di sini.", "オーナー権限は HOP ID(SUPERADMIN_EMAIL)で設定します。ここでは変更できません。", "A propriedade é definida no HOP ID (SUPERADMIN_EMAIL), não aqui.", "La propiedad se define en HOP ID (SUPERADMIN_EMAIL), no aquí.", "La propriété se définit dans HOP ID (SUPERADMIN_EMAIL), pas ici.", "Владелец назначается в HOP ID (SUPERADMIN_EMAIL), а не здесь."},
	"err.owner_only":      {"Only an owner can change roles.", "只有所有者可以更改角色。", "Nur ein Eigentümer kann Rollen ändern.", "Hanya pemilik yang dapat mengubah peran.", "ロールを変更できるのはオーナーだけです。", "Somente um proprietário pode alterar funções.", "Solo un propietario puede cambiar los roles.", "Seul un propriétaire peut modifier les rôles.", "Менять роли может только владелец."},
}

func init() {
	for key, vals := range announceText {
		for i, lang := range langOrder {
			if catalog[lang] == nil {
				catalog[lang] = map[string]string{}
			}
			catalog[lang][key] = vals[i]
		}
	}
}
