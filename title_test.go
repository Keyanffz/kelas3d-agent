package main

import (
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestCodeDetail(t *testing.T) {
	tests := []struct {
		name          string
		w             window
		project, file string
	}{
		{"VS Code", window{"Code.exe", "auth.ts - toko-online - Visual Studio Code"}, "toko-online", "auth.ts"},
		{"VS Code dengan perubahan belum disimpan", window{"code", "● auth.ts - toko-online - Visual Studio Code"}, "toko-online", "auth.ts"},
		{"VS Code dengan profil", window{"code", "main.py - tugas-ml - Data - Visual Studio Code"}, "tugas-ml", "main.py"},
		{"VS Code tanpa folder", window{"code", "catatan.md - Visual Studio Code"}, "", "catatan.md"},
		{"VS Code tanpa editor terbuka", window{"code", "toko-online - Visual Studio Code"}, "", ""},
		{"VS Code tab Welcome", window{"code", "Welcome - toko-online - Visual Studio Code"}, "toko-online", ""},
		{"VS Code diff", window{"code", "auth.ts (Working Tree) - toko-online - Visual Studio Code"}, "toko-online", "auth.ts"},
		{"VS Code remote", window{"code", "server.go - api [SSH: lab] - Visual Studio Code"}, "api", "server.go"},
		{"VS Code workspace", window{"code", "app.vue - kelas (Workspace) - Visual Studio Code"}, "kelas", "app.vue"},
		{"VS Code Insiders", window{"code-insiders", "index.php - laravel - Visual Studio Code - Insiders"}, "laravel", "index.php"},
		{"Cursor", window{"Cursor.exe", "● auth.ts - toko-online - Cursor"}, "toko-online", "auth.ts"},
		{"Windsurf tanpa editor terbuka", window{"windsurf", "toko-online - Windsurf"}, "", ""},
		{"Antigravity", window{"antigravity-ide", "Semester 5 - Antigravity IDE - index.html"}, "Semester 5", "index.html"},
		{"Antigravity belum disimpan", window{"Antigravity.exe", "toko-online - Antigravity - ● auth.ts"}, "toko-online", "auth.ts"},
		{"Antigravity tanpa editor terbuka", window{"antigravity-ide", "Semester 5 - Antigravity IDE"}, "", ""},
		{"Android Studio", window{"studio64.exe", "MyApp – MainActivity.kt [MyApp.app.main]"}, "MyApp", "MainActivity.kt"},
		{"PyCharm", window{"pycharm64.exe", "toko-online – auth.py"}, "toko-online", "auth.py"},
		{"PyCharm dengan path", window{"pycharm", "toko-online [~/kode/toko-online] – src/models/user.py"}, "toko-online", "user.py"},
		{"PyCharm layar awal", window{"pycharm", "Welcome to PyCharm"}, "", ""},
		{"PyCharm format lama", window{"pycharm", "toko-online – auth.py - PyCharm"}, "toko-online", ""},
		{"JupyterLab", window{"firefox", "analisis.ipynb - JupyterLab — Mozilla Firefox"}, "", "analisis.ipynb"},
		{"JupyterLab belum disimpan", window{"chrome.exe", "● analisis.ipynb - JupyterLab - Google Chrome"}, "", "analisis.ipynb"},
		{"JupyterLab launcher", window{"firefox", "Launcher - JupyterLab — Mozilla Firefox"}, "", ""},
		{"Notebook klasik", window{"chrome.exe", "analisis - Jupyter Notebook - Google Chrome"}, "", "analisis.ipynb"},
		{"Notebook halaman daftar", window{"firefox", "Home Page - Select or create a notebook - Jupyter Notebook"}, "", ""},
		{"judul tidak dikenali", window{"Notepad.exe", "auth.ts - Notepad"}, "", ""},
		{"en dash di aplikasi bukan JetBrains", window{"firefox", "Jupyter – Wikipedia — Mozilla Firefox"}, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			project, file := codeDetail(tt.w)
			if project != tt.project || file != tt.file {
				t.Errorf("codeDetail(%q) = %q, %q; mau %q, %q", tt.w.Title, project, file, tt.project, tt.file)
			}
		})
	}
}

func TestLanguage(t *testing.T) {
	for file, want := range map[string]string{
		"auth.ts": "TypeScript", "App.TSX": "TypeScript", "analisis.ipynb": "Python", "main.PY": "Python",
		"index.blade.php": "PHP", "Main.java": "Java", "catatan.md": "", "Makefile": "",
	} {
		if got := language(file); got != want {
			t.Errorf("language(%q) = %q, mau %q", file, got, want)
		}
	}
}

// Judul browser dan aplikasi chat tidak boleh ikut terkirim dalam bentuk apa pun.
func TestReportNeverSendsTitles(t *testing.T) {
	rules := []appRule{
		{MatchOn: "title", Pattern: "ChatGPT", Activity: "vibe_coding", Label: "ChatGPT", Priority: 30},
		{MatchOn: "title", Pattern: "Jupyter", Activity: "ngoding", Label: "Jupyter", Priority: 20},
		{MatchOn: "process", Pattern: "code", Activity: "ngoding", Label: "VS Code", Priority: 10},
		{MatchOn: "process", Pattern: "firefox", Activity: "browsing", Label: "Firefox", Priority: 5},
		{MatchOn: "process", Pattern: "chrome.exe", Activity: "browsing", Label: "Chrome", Priority: 5},
	}
	windows := []window{
		{"firefox", "Rekening Saya - Internet Banking — Mozilla Firefox"},
		{"chrome.exe", "Kotak Masuk (3) - budi@gmail.com - Gmail - Google Chrome"},
		{"chrome.exe", "Cara bikin skripsi cepat - ChatGPT - Google Chrome"},
		{"firefox", "jupyter tutorial - Penelusuran Google — Mozilla Firefox"},
		{"WhatsApp.exe", "WhatsApp - Grup Kelas Rahasia"},
		{"Discord.exe", "#umum | Server Bengkod - Discord"},
		{"telegram-desktop", "Pacar ❤️"},
		{"code", "Gmail Rahasia - Visual Studio Code Fake — Mozilla Firefox"},
	}
	// Daftar aplikasi terbuka cuma boleh berisi label dan nama proses.
	b, _ := json.Marshal(openApps(rules, nil, windows))
	for _, w := range windows {
		for _, word := range strings.Fields(w.Title) {
			lower := strings.ToLower(word)
			if len(word) > 3 && strings.Contains(strings.ToLower(string(b)), lower) && !strings.Contains(strings.ToLower(w.Process), lower) &&
				!slices.ContainsFunc(rules, func(r appRule) bool { return strings.EqualFold(r.Label, word) }) {
				t.Errorf("open_apps: kata %q dari judul ikut terkirim: %s", word, b)
			}
		}
	}

	for _, w := range windows {
		r := buildReport(rules, w, 0, "192.168.10.23", time.Now())
		b, _ := json.Marshal(r)
		if r.Project != nil || r.FileName != nil || r.Language != nil {
			t.Errorf("%q: project/file/bahasa terisi: %s", w.Title, b)
		}
		// Nama proses dan label aplikasi memang dikirim; selain itu tidak boleh ada kata dari judul.
		allowed := strings.ToLower(w.Process)
		if r.AppLabel != nil {
			allowed += " " + strings.ToLower(*r.AppLabel)
		}
		for _, word := range strings.Fields(w.Title) {
			if len(word) > 3 && strings.Contains(string(b), word) && !strings.Contains(allowed, strings.ToLower(word)) {
				t.Errorf("%q: kata %q dari judul ikut terkirim: %s", w.Title, word, b)
			}
		}
	}
}

func TestOpenApps(t *testing.T) {
	rules := []appRule{
		{MatchOn: "title", Pattern: "ChatGPT", Activity: "vibe_coding", Label: "ChatGPT", Priority: 30},
		{MatchOn: "process", Pattern: "code", Activity: "ngoding", Label: "VS Code", Priority: 10},
		{MatchOn: "process", Pattern: "chrome", Activity: "browsing", Label: "Chrome", Priority: 5},
	}
	focused := "VS Code"
	got := openApps(rules, &focused, []window{
		{"code", "auth.ts - toko - Visual Studio Code"}, // aplikasi yang sedang fokus
		{"chrome", "Kotak Masuk - Gmail - Google Chrome"},
		{"chrome", "Tugas - ChatGPT - Google Chrome"},
		{"chrome", "YouTube - Google Chrome"}, // Chrome sudah ada
		{"Discord.exe", "#umum | Bengkod - Discord"},
	})
	want := []openApp{{"browsing", "Chrome"}, {"vibe_coding", "ChatGPT"}, {"lainnya", "Discord"}}
	if !slices.Equal(got, want) {
		t.Errorf("openApps = %v, mau %v", got, want)
	}

	var banyak []window
	for i := range 20 {
		banyak = append(banyak, window{Process: fmt.Sprintf("app%d.exe", i), Title: "x"})
	}
	if n := len(openApps(rules, nil, banyak)); n != maxOpenApps {
		t.Errorf("%d aplikasi, mau dibatasi %d", n, maxOpenApps)
	}

	// nil: desktop belum didukung (dikirim null); kosong: tidak ada jendela lain (dikirim []).
	if openApps(rules, nil, nil) != nil {
		t.Error("tanpa daftar jendela harus nil")
	}
	if b, _ := json.Marshal(openApps(rules, &focused, []window{{"code", "x"}})); string(b) != "[]" {
		t.Errorf("tanpa aplikasi lain = %s, mau []", b)
	}
}

func TestBuildReportCoding(t *testing.T) {
	rules := []appRule{{MatchOn: "process", Pattern: "code", Activity: "ngoding", Label: "VS Code", Priority: 10}}
	r := buildReport(rules, window{"code", "● auth.ts - toko-online - Visual Studio Code"}, 12*time.Second+900*time.Millisecond, "", time.Now())
	if *r.Project != "toko-online" || *r.FileName != "auth.ts" || *r.Language != "TypeScript" || r.IdleSeconds != 12 || r.LocalIP != nil {
		b, _ := json.Marshal(r)
		t.Errorf("laporan: %s", b)
	}

	long := strings.Repeat("é", 300) + ".py"
	r = buildReport(rules, window{"code", long + " - toko - Visual Studio Code"}, 0, "", time.Now())
	if n := len([]rune(*r.FileName)); n != 255 {
		t.Errorf("nama file %d karakter, mau dipotong 255", n)
	}
}
