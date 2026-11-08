package main

import (
	"fmt"
	"testing"
	"time"
)

func ptr[T any](v T) *T { return &v }

func TestMatchApp(t *testing.T) {
	other := "macos"
	if osName == "macos" {
		other = "windows"
	}
	rules := []appRule{
		{MatchOn: "title", Pattern: "ChatGPT", Activity: "vibe_coding", Label: "ChatGPT", Priority: 30},
		{MatchOn: "title", Pattern: "Jupyter", Activity: "ngoding", Label: "Jupyter", Priority: 20},
		{MatchOn: "process", Pattern: "code", OS: ptr(osName), Activity: "ngoding", Label: "VS Code", Priority: 10},
		{MatchOn: "process", Pattern: "Code.exe", OS: ptr(osName), Activity: "ngoding", Label: "VS Code", Priority: 10},
		{MatchOn: "process", Pattern: "steam", OS: ptr(other), Activity: "main_game", Label: "Steam", Priority: 10},
		{MatchOn: "process", Pattern: "firefox", Activity: "browsing", Label: "Firefox", Priority: 5},
		{MatchOn: "process", Pattern: "FIREFOX", Activity: "lainnya", Label: "Firefox kedua", Priority: 5},
		{MatchOn: "title", Pattern: "", Activity: "main_game", Label: "Pola kosong", Priority: 100},
	}

	tests := []struct {
		name            string
		w               window
		activity, label string
	}{
		{"nama proses sama, huruf besar-kecil diabaikan", window{Process: "Code"}, "ngoding", "VS Code"},
		{"nama proses Windows dengan .exe", window{Process: "code.EXE"}, "ngoding", "VS Code"},
		{"nama proses harus sama persis", window{Process: "code-insiders"}, "lainnya", "code-insiders"},
		{"aturan OS lain diabaikan", window{Process: "steam"}, "lainnya", "steam"},
		{"judul mengandung pola", window{Process: "chrome", Title: "Notebook - jupyter lab"}, "ngoding", "Jupyter"},
		{"priority terbesar menang", window{Process: "firefox", Title: "ChatGPT — Mozilla Firefox"}, "vibe_coding", "ChatGPT"},
		{"priority sama, yang lebih dulu menang", window{Process: "firefox", Title: "Beranda"}, "browsing", "Firefox"},
		{"tidak cocok: label dari nama proses tanpa .exe", window{Process: "Notepad.exe", Title: "catatan.txt"}, "lainnya", "Notepad"},
		{"tanpa jendela aktif", window{}, "lainnya", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			activity, label := matchApp(rules, tt.w)
			if activity != tt.activity || label != tt.label {
				t.Errorf("matchApp(%+v) = %q, %q; mau %q, %q", tt.w, activity, label, tt.activity, tt.label)
			}
		})
	}
}

func TestInClass(t *testing.T) {
	rooms := []classroom{
		{Slug: "developer", Schedules: []schedule{
			{Type: "kelas", DayOfWeek: 1, StartsAt: "08:00", EndsAt: "16:00"},
			{Type: "istirahat", DayOfWeek: 1, StartsAt: "12:00", EndsAt: "13:00"},
			{Type: "istirahat", DayOfWeek: 2, StartsAt: "12:00", EndsAt: "13:00"},
		}},
		{Slug: "data-science", Schedules: []schedule{
			{Type: "kelas", DayOfWeek: 7, StartsAt: "10:00", EndsAt: "11:00"},
		}},
	}

	// 9 November 2026 hari Senin.
	at := func(day int, clock string, zone *time.Location) time.Time {
		tm, err := time.ParseInLocation("2006-01-02 15:04:05", fmt.Sprintf("2026-11-%02d %s", day, clock), zone)
		if err != nil {
			t.Fatal(err)
		}
		return tm
	}
	tests := []struct {
		name string
		now  time.Time
		want bool
	}{
		{"sebelum kelas mulai", at(9, "07:59:59", wib), false},
		{"tepat kelas mulai", at(9, "08:00:00", wib), true},
		{"jam istirahat tetap di dalam jadwal kelas", at(9, "12:30:00", wib), true},
		{"detik terakhir kelas", at(9, "15:59:59", wib), true},
		{"tepat kelas selesai", at(9, "16:00:00", wib), false},
		{"hari tanpa jadwal kelas, cuma istirahat", at(10, "12:30:00", wib), false},
		{"Minggu = 7", at(8, "10:30:00", wib), true},
		{"jam laptop UTC dikonversi ke WIB", at(9, "01:30:00", time.UTC), true},
		{"UTC 09:30 = WIB 16:30", at(9, "09:30:00", time.UTC), false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := inClass(rooms, tt.now); got != tt.want {
				t.Errorf("inClass(%s) = %v, mau %v", tt.now, got, tt.want)
			}
		})
	}
}
