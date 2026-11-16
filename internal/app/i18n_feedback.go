package app

// Labels of the Feedback pages. One line per key, in the order of langOrder: en, zh-CN, de, id, ja, pt-BR, es, fr, ru.
var feedbackText = map[string][9]string{
	"nav.feedback":    {"Feedback", "反馈", "Feedback", "Masukan", "フィードバック", "Feedback", "Comentarios", "Retours", "Отзывы"},
	"fb.title":        {"Feedback", "反馈", "Feedback", "Masukan", "フィードバック", "Feedback", "Comentarios", "Retours", "Отзывы"},
	"fb.none":         {"No feedback yet.", "暂无反馈。", "Noch kein Feedback.", "Belum ada masukan.", "フィードバックはありません。", "Nenhum feedback.", "Aún no hay comentarios.", "Aucun retour.", "Отзывов пока нет."},
	"fb.col.kind":     {"Type", "类型", "Art", "Jenis", "種類", "Tipo", "Tipo", "Type", "Тип"},
	"fb.col.message":  {"Message", "内容", "Nachricht", "Pesan", "内容", "Mensagem", "Mensaje", "Message", "Сообщение"},
	"fb.col.from":     {"From", "发送者", "Von", "Dari", "送信者", "De", "De", "De", "От"},
	"fb.col.received": {"Received", "收到时间", "Eingegangen", "Diterima", "受信日時", "Recebido", "Recibido", "Reçu le", "Получено"},
	"fb.col.status":   {"Status", "状态", "Status", "Status", "状態", "Status", "Estado", "Statut", "Статус"},
	"fb.kind.problem": {"Problem", "问题", "Problem", "Masalah", "問題", "Problema", "Problema", "Problème", "Проблема"},
	"fb.kind.idea":    {"Idea or suggestion", "想法或建议", "Idee oder Vorschlag", "Ide atau saran", "アイデア・提案", "Ideia ou sugestão", "Idea o sugerencia", "Idée ou suggestion", "Идея или предложение"},
	"fb.kind.other":   {"Other", "其他", "Sonstiges", "Lainnya", "その他", "Outro", "Otro", "Autre", "Другое"},
	"fb.status.new":   {"New", "新", "Neu", "Baru", "新規", "Novo", "Nuevo", "Nouveau", "Новое"},
	"fb.status.read":  {"Read", "已读", "Gelesen", "Dibaca", "既読", "Lido", "Leído", "Lu", "Прочитано"},
	"fb.status.done":  {"Done", "已完成", "Erledigt", "Selesai", "完了", "Concluído", "Hecho", "Terminé", "Готово"},
	"fb.guest":        {"Guest", "访客", "Gast", "Tamu", "ゲスト", "Visitante", "Invitado", "Invité", "Гость"},
	"fb.has_image":    {"Image", "图片", "Bild", "Gambar", "画像", "Imagem", "Imagen", "Image", "Изображение"},
	"fb.d.page":       {"Screen", "页面", "Bildschirm", "Layar", "画面", "Tela", "Pantalla", "Écran", "Экран"},
	"fb.d.version":    {"App version", "应用版本", "App-Version", "Versi aplikasi", "アプリのバージョン", "Versão do app", "Versión de la app", "Version de l'app", "Версия приложения"},
	"fb.d.ip":         {"IP address", "IP 地址", "IP-Adresse", "Alamat IP", "IPアドレス", "Endereço IP", "Dirección IP", "Adresse IP", "IP-адрес"},
	"fb.d.ip_gone":    {"Removed after 30 days", "30 天后已清除", "Nach 30 Tagen gelöscht", "Dihapus setelah 30 hari", "30日後に削除済み", "Removido após 30 dias", "Eliminada tras 30 días", "Supprimée après 30 jours", "Удалён через 30 дней"},
	"fb.d.agent":      {"Browser", "浏览器", "Browser", "Peramban", "ブラウザ", "Navegador", "Navegador", "Navigateur", "Браузер"},
	"fb.d.viewport":   {"Window size", "窗口大小", "Fenstergröße", "Ukuran jendela", "ウィンドウサイズ", "Tamanho da janela", "Tamaño de ventana", "Taille de la fenêtre", "Размер окна"},
	"fb.d.locale":     {"Language", "语言", "Sprache", "Bahasa", "言語", "Idioma", "Idioma", "Langue", "Язык"},
	"fb.d.read_at":    {"Read at", "阅读时间", "Gelesen am", "Dibaca pada", "既読日時", "Lido em", "Leído el", "Lu le", "Прочитано"},
	"fb.d.done_at":    {"Done at", "完成时间", "Erledigt am", "Selesai pada", "完了日時", "Concluído em", "Hecho el", "Terminé le", "Завершено"},
	"fb.d.image":      {"Screenshot", "截图", "Screenshot", "Tangkapan layar", "スクリーンショット", "Captura de tela", "Captura de pantalla", "Capture d'écran", "Скриншот"},
	"fb.d.no_image":   {"No image was sent.", "未发送图片。", "Es wurde kein Bild gesendet.", "Tidak ada gambar yang dikirim.", "画像は送信されていません。", "Nenhuma imagem foi enviada.", "No se envió ninguna imagen.", "Aucune image n'a été envoyée.", "Изображение не отправлено."},
	"fb.d.show_image": {"Show image", "显示图片", "Bild anzeigen", "Tampilkan gambar", "画像を表示", "Mostrar imagem", "Mostrar imagen", "Afficher l'image", "Показать изображение"},
	"fb.d.image_alt":  {"Screenshot sent with the report", "随报告发送的截图", "Mit dem Bericht gesendeter Screenshot", "Tangkapan layar yang dikirim bersama laporan", "報告と一緒に送られたスクリーンショット", "Captura enviada com o relato", "Captura enviada con el informe", "Capture envoyée avec le signalement", "Скриншот, отправленный с сообщением"},
	"fb.act.done":     {"Mark done", "标记为完成", "Als erledigt markieren", "Tandai selesai", "完了にする", "Marcar como concluído", "Marcar como hecho", "Marquer comme terminé", "Отметить готовым"},
	"fb.act.reopen":   {"Reopen", "重新打开", "Wieder öffnen", "Buka lagi", "再開する", "Reabrir", "Reabrir", "Rouvrir", "Открыть снова"},
	"fb.err.old_hub":  {"This HOP ID does not have the feedback inbox yet. Update it first.", "此 HOP ID 尚无反馈收件箱,请先更新。", "Diese HOP-ID-Version hat noch keinen Feedback-Eingang. Bitte zuerst aktualisieren.", "HOP ID ini belum punya kotak masukan. Perbarui dulu.", "この HOP ID にはまだフィードバック受信箱がありません。先に更新してください。", "Este HOP ID ainda não tem a caixa de feedback. Atualize primeiro.", "Este HOP ID aún no tiene la bandeja de comentarios. Actualízalo primero.", "Ce HOP ID n'a pas encore la boîte de retours. Mettez-le d'abord à jour.", "В этом HOP ID ещё нет ящика отзывов. Сначала обновите его."},
	"fb.err.gone":     {"This report no longer exists. It may have been deleted.", "该报告已不存在,可能已被删除。", "Dieser Bericht existiert nicht mehr. Er wurde vielleicht gelöscht.", "Laporan ini sudah tidak ada. Mungkin sudah dihapus.", "この報告はもう存在しません。削除された可能性があります。", "Este relato não existe mais. Pode ter sido excluído.", "Este informe ya no existe. Puede que se haya eliminado.", "Ce signalement n'existe plus. Il a peut-être été supprimé.", "Этого сообщения больше нет. Возможно, оно удалено."},
	"fb.err.image":    {"The image could not be loaded.", "无法加载图片。", "Das Bild konnte nicht geladen werden.", "Gambar tidak dapat dimuat.", "画像を読み込めませんでした。", "Não foi possível carregar a imagem.", "No se pudo cargar la imagen.", "L'image n'a pas pu être chargée.", "Не удалось загрузить изображение."},
}

func init() {
	for key, vals := range feedbackText {
		for i, lang := range langOrder {
			if catalog[lang] == nil {
				catalog[lang] = map[string]string{}
			}
			catalog[lang][key] = vals[i]
		}
	}
}
