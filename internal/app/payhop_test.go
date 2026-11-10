package app

import "testing"

// PayHop reports accounts and sessions like FileHop, and also how many payment methods and bills
// its people saved. Those two numbers never reach the dashboard: Imperium shows nothing of that data.
func TestPayHopKPIOrderHidesPaymentCounts(t *testing.T) {
	a, _ := newApp(t, newFakeHop(t))
	all, attention := a.kpis("en", "payhop", map[string]int64{
		"users_suspended": 0, "sessions_live": 3, "users": 8, "users_active_30d": 5, "methods_saved": 4, "bills_saved": 40,
	})
	want := []string{"users", "users_active_30d", "sessions_live", "users_suspended"}
	if len(all) != len(want) {
		t.Fatalf("got %d KPIs %+v, want %v", len(all), all, want)
	}
	for i, k := range want {
		if all[i].Key != k {
			t.Errorf("KPI %d is %q, want %q", i, all[i].Key, k)
		}
	}
	if len(attention) != 0 {
		t.Errorf("attention = %+v, want none", attention)
	}
}

func TestAppMarkNamesEveryFamilyApp(t *testing.T) {
	for _, slug := range []string{"linkhop", "biohop", "pixelhop", "notehop", "filehop", "payhop"} {
		if got, want := appMark(slug), "/static/hop/apps/"+slug+"/mark.svg"; got != want {
			t.Errorf("appMark(%q) = %q, want %q", slug, got, want)
		}
	}
	if got := appMark("unknown"); got != "/static/imperium.svg" {
		t.Errorf("an unknown app gets Imperium's own mark, got %q", got)
	}
}
