package app

// Words of the reports list that are not about an action: what a moderator reads in the subject cell. Nine
// languages in the order of langOrder: en, zh-CN, de, id, ja, pt-BR, es, fr, ru.

var reportsKeys = []string{"reports.page_deleted"}

var reportsVals = map[string][]string{
	"en":    {"page deleted"},
	"zh-CN": {"页面已删除"},
	"de":    {"Seite gelöscht"},
	"id":    {"halaman sudah dihapus"},
	"ja":    {"ページは削除済み"},
	"pt-BR": {"página excluída"},
	"es":    {"página eliminada"},
	"fr":    {"page supprimée"},
	"ru":    {"страница удалена"},
}

func init() {
	for lang, vals := range reportsVals {
		if catalog[lang] == nil {
			catalog[lang] = map[string]string{}
		}
		for i, k := range reportsKeys {
			catalog[lang][k] = vals[i]
		}
	}
}
