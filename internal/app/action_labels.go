package app

import "strings"

// actionLabels name the protocol actions in the nine languages, in the order of langOrder:
// en, zh-CN, de, id, ja, pt-BR, es, fr, ru. A new action keeps a readable fallback.
var actionLabels = map[string][9]string{
	"delete":           {"Delete", "删除", "Löschen", "Hapus", "削除", "Excluir", "Eliminar", "Supprimer", "Удалить"},
	"purge":            {"Permanently delete", "永久删除", "Endgültig löschen", "Hapus permanen", "完全に削除", "Excluir permanentemente", "Eliminar permanentemente", "Supprimer définitivement", "Удалить навсегда"},
	"takedown":         {"Take down", "下架", "Offline nehmen", "Tarik konten", "公開停止", "Retirar do ar", "Retirar", "Retirer", "Изъять"},
	"ban_ip":           {"Block IP", "封禁 IP", "IP sperren", "Blokir IP", "IP をブロック", "Bloquear IP", "Bloquear IP", "Bloquer l'IP", "Заблокировать IP"},
	"hide":             {"Hide", "隐藏", "Ausblenden", "Sembunyikan", "非表示にする", "Ocultar", "Ocultar", "Masquer", "Скрыть"},
	"unhide":           {"Show", "显示", "Einblenden", "Tampilkan", "表示する", "Mostrar", "Mostrar", "Afficher", "Показать"},
	"disable":          {"Disable", "停用", "Deaktivieren", "Nonaktifkan", "無効にする", "Desativar", "Desactivar", "Désactiver", "Отключить"},
	"enable":           {"Enable", "启用", "Aktivieren", "Aktifkan", "有効にする", "Ativar", "Activar", "Activer", "Включить"},
	"publish":          {"Publish", "发布", "Veröffentlichen", "Terbitkan", "公開する", "Publicar", "Publicar", "Publier", "Опубликовать"},
	"unpublish":        {"Unpublish", "取消发布", "Veröffentlichung aufheben", "Batalkan publikasi", "公開を取り消す", "Cancelar publicação", "Anular publicación", "Dépublier", "Снять с публикации"},
	"transfer":         {"Transfer ownership", "转移所有权", "Eigentum übertragen", "Pindahkan kepemilikan", "所有権を移す", "Transferir propriedade", "Transferir propiedad", "Transférer la propriété", "Передать владение"},
	"resolve":          {"Resolve", "标记为已解决", "Erledigen", "Selesaikan", "解決する", "Resolver", "Resolver", "Résoudre", "Решить"},
	"dismiss":          {"Dismiss", "忽略", "Verwerfen", "Abaikan", "却下する", "Dispensar", "Descartar", "Ignorer", "Отклонить"},
	"confirm":          {"Confirm", "确认", "Bestätigen", "Konfirmasi", "確認する", "Confirmar", "Confirmar", "Confirmer", "Подтвердить"},
	"restore":          {"Restore", "恢复", "Wiederherstellen", "Pulihkan", "復元する", "Restaurar", "Restaurar", "Restaurer", "Восстановить"},
	"mark_nsfw":        {"Mark adult", "标记为成人内容", "Als Inhalt für Erwachsene markieren", "Tandai dewasa", "成人向けにする", "Marcar como adulto", "Marcar como adulto", "Marquer comme adulte", "Пометить как 18+"},
	"unmark_nsfw":      {"Remove adult label", "移除成人标记", "Erwachsenen-Kennzeichnung entfernen", "Hapus label dewasa", "成人向けラベルを外す", "Remover rótulo adulto", "Quitar etiqueta de adulto", "Retirer l'étiquette adulte", "Снять пометку 18+"},
	"mark_sensitive":   {"Mark sensitive", "标记为敏感内容", "Als sensibel markieren", "Tandai sensitif", "センシティブにする", "Marcar como sensível", "Marcar como sensible", "Marquer comme sensible", "Пометить как чувствительный"},
	"unmark_sensitive": {"Remove sensitive label", "移除敏感标记", "Sensibel-Kennzeichnung entfernen", "Hapus label sensitif", "センシティブラベルを外す", "Remover rótulo sensível", "Quitar etiqueta de sensible", "Retirer l'étiquette sensible", "Снять пометку чувствительного"},
	"clear_custom_css": {"Clear custom CSS", "清除自定义 CSS", "Eigenes CSS löschen", "Hapus CSS kustom", "カスタム CSS を削除", "Limpar CSS personalizado", "Borrar CSS personalizado", "Effacer le CSS personnalisé", "Очистить свой CSS"},
	"investigate":      {"Investigate", "调查", "Untersuchen", "Selidiki", "調査する", "Investigar", "Investigar", "Enquêter", "Расследовать"},
	"close":            {"Close report", "关闭举报", "Meldung schließen", "Tutup laporan", "報告を閉じる", "Fechar denúncia", "Cerrar denuncia", "Clore le signalement", "Закрыть жалобу"},
	"rescan":           {"Scan again", "重新扫描", "Erneut scannen", "Pindai ulang", "再スキャン", "Verificar novamente", "Volver a analizar", "Analyser à nouveau", "Сканировать снова"},
	"freeze":           {"Freeze", "冻结", "Einfrieren", "Bekukan", "凍結する", "Congelar", "Congelar", "Geler", "Заморозить"},
	"set_expiry":       {"Set expiry", "设置到期时间", "Ablaufdatum setzen", "Atur kedaluwarsa", "有効期限を設定", "Definir validade", "Definir caducidad", "Définir l'expiration", "Задать срок действия"},
	"release_slug":     {"Release slug", "释放短链接", "Slug freigeben", "Lepaskan slug", "スラッグを解放", "Liberar slug", "Liberar slug", "Libérer le slug", "Освободить slug"},
	"suspend":          {"Suspend", "暂停账号", "Sperren", "Tangguhkan", "停止する", "Suspender", "Suspender", "Suspendre", "Приостановить"},
	"activate":         {"Activate", "恢复账号", "Entsperren", "Aktifkan akun", "有効化する", "Reativar conta", "Reactivar cuenta", "Réactiver", "Активировать"},
	"revoke_sessions":  {"Sign out everywhere", "退出所有设备", "Überall abmelden", "Keluarkan dari semua sesi", "すべてのセッションからログアウト", "Sair de todas as sessões", "Cerrar todas las sesiones", "Déconnecter partout", "Выйти на всех устройствах"},
	"revoke_tokens":    {"Revoke API tokens", "撤销 API 令牌", "API-Token widerrufen", "Cabut token API", "API トークンを取り消す", "Revogar tokens de API", "Revocar tokens de API", "Révoquer les jetons API", "Отозвать API-токены"},
	"ban":              {"Ban", "封禁", "Bannen", "Blokir", "ブロック", "Banir", "Bloquear", "Bannir", "Забанить"},
	"remove":           {"Remove", "移除", "Entfernen", "Hapus dari daftar", "取り除く", "Remover", "Quitar", "Retirer", "Убрать"},
	"release":          {"Release", "释放", "Freigeben", "Lepaskan", "解放", "Liberar", "Liberar", "Libérer", "Освободить"},
}

func actionLabel(l, action string) string {
	if label, ok := actionLabels[action]; ok {
		for i, code := range langOrder {
			if code == l {
				return label[i]
			}
		}
		return label[0]
	}
	return strings.ReplaceAll(action, "_", " ")
}

func confirmAction(action string) bool {
	switch action {
	case "delete", "purge", "takedown", "ban_ip", "suspend", "disable", "hide", "unpublish", "transfer", "clear_custom_css", "resolve", "confirm", "freeze", "set_expiry", "release_slug":
		return true
	}
	return false
}
