package app

import "testing"

// FileHop reports accounts and sessions only (files never reach its server);
// its dashboard puts them in a fixed order, and a suspended account is work.
func TestFileHopKPIOrder(t *testing.T) {
	a, _ := newApp(t, newFakeHop(t))
	all, attention := a.kpis("filehop", map[string]int64{
		"users_suspended": 1, "sessions_live": 9, "users": 12, "users_active_30d": 11,
	})
	want := []string{"users", "users_active_30d", "sessions_live", "users_suspended"}
	if len(all) != len(want) {
		t.Fatalf("got %d KPIs, want %d", len(all), len(want))
	}
	for i, k := range want {
		if all[i].Key != k {
			t.Errorf("KPI %d is %q, want %q", i, all[i].Key, k)
		}
	}
	if len(attention) != 1 || attention[0].Key != "users_suspended" {
		t.Errorf("attention = %+v, want users_suspended only", attention)
	}
}
