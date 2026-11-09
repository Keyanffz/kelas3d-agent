package main

import (
	"slices"
	"testing"
)

func TestHyprlandClients(t *testing.T) {
	// pid 0 supaya nama proses diambil dari class, tanpa membaca /proc.
	out := []byte(`[
		{"class": "firefox", "title": "Docs", "pid": 0, "mapped": true, "hidden": false, "focusHistoryID": 2},
		{"class": "code", "title": "auth.ts - toko - Visual Studio Code", "pid": 0, "mapped": true, "hidden": false, "focusHistoryID": 0},
		{"class": "kitty", "title": "", "pid": 0, "mapped": true, "hidden": false, "focusHistoryID": 1},
		{"class": "steam", "title": "Steam", "pid": 0, "mapped": true, "hidden": true, "focusHistoryID": 3},
		{"class": "obsidian", "title": "Catatan", "pid": 0, "mapped": false, "hidden": false, "focusHistoryID": 4}
	]`)
	got, err := hyprlandClients(out)
	want := []window{{"code", "auth.ts - toko - Visual Studio Code"}, {"firefox", "Docs"}}
	if err != nil || !slices.Equal(got, want) {
		t.Errorf("hyprlandClients = %v, %v; mau %v", got, err, want)
	}
	if got, _ := hyprlandClients([]byte(`[]`)); got == nil {
		t.Error("tanpa jendela harus daftar kosong, bukan nil")
	}
}
