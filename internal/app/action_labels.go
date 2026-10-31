package app

import "strings"

// Known protocol actions have readable labels; new actions keep a readable fallback.
func (a *App) actionLabel(action string) string {
	labels := map[string][2]string{
		"delete": {"Delete", "Hapus"}, "purge": {"Permanently delete", "Hapus permanen"},
		"takedown": {"Take down", "Tarik konten"}, "ban_ip": {"Block IP", "Blokir IP"},
		"hide": {"Hide", "Sembunyikan"}, "unhide": {"Show", "Tampilkan"},
		"disable": {"Disable", "Nonaktifkan"}, "enable": {"Enable", "Aktifkan"},
		"publish": {"Publish", "Terbitkan"}, "unpublish": {"Unpublish", "Batalkan publikasi"},
		"transfer": {"Transfer ownership", "Pindahkan kepemilikan"},
		"resolve":  {"Resolve", "Selesaikan"}, "dismiss": {"Dismiss", "Abaikan"},
		"confirm": {"Confirm", "Konfirmasi"}, "restore": {"Restore", "Pulihkan"},
		"mark_nsfw": {"Mark adult", "Tandai dewasa"}, "unmark_nsfw": {"Remove adult label", "Hapus label dewasa"},
		"mark_sensitive": {"Mark sensitive", "Tandai sensitif"}, "unmark_sensitive": {"Remove sensitive label", "Hapus label sensitif"},
		"clear_custom_css": {"Clear custom CSS", "Hapus CSS kustom"},
		"investigate":      {"Investigate", "Selidiki"}, "close": {"Close report", "Tutup laporan"},
		"rescan": {"Scan again", "Pindai ulang"}, "freeze": {"Freeze", "Bekukan"},
		"set_expiry": {"Set expiry", "Atur kedaluwarsa"}, "release_slug": {"Release slug", "Lepaskan slug"},
	}
	if label, ok := labels[action]; ok {
		if a.lang() == "id" {
			return label[1]
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
