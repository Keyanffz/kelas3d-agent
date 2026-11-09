package main

import (
	"path/filepath"
	"strings"
	"unicode/utf8"
)

// Bahasa dari ekstensi file. Ekstensi lain dikirim tanpa bahasa.
var languages = map[string]string{
	".py": "Python", ".ipynb": "Python",
	".js": "JavaScript", ".jsx": "JavaScript", ".mjs": "JavaScript", ".cjs": "JavaScript",
	".ts": "TypeScript", ".tsx": "TypeScript",
	".php": "PHP", ".java": "Java", ".kt": "Kotlin", ".dart": "Dart", ".go": "Go",
	".c": "C", ".h": "C", ".cpp": "C++", ".cc": "C++", ".hpp": "C++", ".cs": "C#",
	".rb": "Ruby", ".rs": "Rust", ".swift": "Swift", ".r": "R", ".sql": "SQL", ".sh": "Shell",
	".html": "HTML", ".css": "CSS", ".scss": "SCSS", ".vue": "Vue", ".svelte": "Svelte",
}

func language(file string) string {
	return languages[strings.ToLower(filepath.Ext(file))]
}

// Akhiran judul tiap edisi VS Code.
var vscodeSuffixes = []string{" - Visual Studio Code", " - Visual Studio Code - Insiders", " - Code - OSS", " - VSCodium"}

// Nama aplikasi turunan VS Code di judul. Cursor dan Windsurf mengikuti VS Code, Antigravity menaruhnya di
// tengah: "Semester 5 - Antigravity IDE - index.html". "Antigravity IDE" dicek sebelum "Antigravity".
var vscodeForks = []string{" - Antigravity IDE", " - Antigravity", " - Cursor", " - Windsurf"}

// Nama proses IDE JetBrains. Formatnya sama dengan PyCharm.
var jetbrains = []string{"pycharm", "idea", "webstorm", "phpstorm", "goland", "clion", "rider", "rubymine", "datagrip", "studio"}

// codeDetail mengambil project dan nama file dari judul jendela aplikasi coding. Pola yang tidak dikenali
// menghasilkan string kosong, jadi judul mentah tidak pernah ikut terkirim.
func codeDetail(w window) (project, file string) {
	title := strings.TrimSpace(w.Title)
	for _, suffix := range vscodeSuffixes {
		if rest, ok := strings.CutSuffix(title, suffix); ok {
			return vscodeTitle(rest)
		}
	}
	for _, app := range vscodeForks {
		if rest, ok := strings.CutSuffix(title, app); ok {
			return vscodeTitle(rest)
		}
		if project, file, ok := strings.Cut(title, app+" - "); ok {
			return cleanName(strings.TrimPrefix(project, "●")), fileName(strings.TrimPrefix(file, "●"))
		}
	}
	if name, ok := jupyterTitle(title); ok {
		return "", name
	}
	process := strings.ToLower(w.Process)
	for _, ide := range jetbrains {
		if strings.Contains(process, ide) {
			return jetbrainsTitle(title)
		}
	}
	return "", ""
}

// VS Code: "● auth.ts - toko-online - Visual Studio Code". ● berarti ada perubahan yang belum disimpan, dan
// bagian ketiga (kalau ada) nama profil. Judul satu bagian cuma dipakai kalau berupa file, karena
// "toko-online" (folder tanpa editor) dan "Welcome" (editor tanpa folder) tidak bisa dibedakan.
func vscodeTitle(rest string) (project, file string) {
	parts := strings.Split(strings.TrimSpace(strings.TrimPrefix(rest, "●")), " - ")
	if len(parts) == 1 {
		return "", fileName(parts[0])
	}
	return cleanName(parts[1]), fileName(parts[0])
}

// Jupyter di browser: "analisis.ipynb - JupyterLab — Mozilla Firefox", atau Notebook klasik yang menulis
// nama tanpa ekstensi: "analisis - Jupyter Notebook - Google Chrome".
func jupyterTitle(title string) (string, bool) {
	for _, marker := range []string{" - JupyterLab", " - Jupyter Notebook"} {
		i := strings.Index(title, marker)
		if i <= 0 {
			continue
		}
		name := strings.TrimLeft(title[:i], "●*() ")
		if strings.Contains(name, " - ") {
			return "", true // halaman daftar file, bukan notebook
		}
		if marker == " - Jupyter Notebook" && filepath.Ext(name) == "" {
			name += ".ipynb"
		}
		return fileName(name), true
	}
	return "", false
}

// PyCharm: "toko-online – auth.py" dengan en dash. Project kadang diikuti path ("toko-online [~/kode]")
// dan file kadang ditulis bersama foldernya ("src/auth.py").
func jetbrainsTitle(title string) (project, file string) {
	parts := strings.Split(title, " – ")
	if len(parts) < 2 {
		return "", ""
	}
	return cleanName(parts[0]), fileName(parts[len(parts)-1])
}

// fileName: nama file tanpa folder dan keterangan seperti " (Working Tree)". Bagian judul tanpa ekstensi
// (Welcome, Settings, Untitled-1) bukan file.
func fileName(s string) string {
	s = cleanName(s)
	s = s[strings.LastIndexAny(s, `/\`)+1:]
	if ext := filepath.Ext(s); ext == "" || strings.ContainsRune(ext, ' ') {
		return ""
	}
	return s
}

// cleanName membuang keterangan di belakang nama: " (Workspace)", " [SSH: server]", " [~/kode/toko]".
func cleanName(s string) string {
	if i := strings.IndexAny(s, "(["); i > 0 {
		s = s[:i]
	}
	return strings.TrimSpace(s)
}

// limit memotong ke n karakter, batas panjang kolom di API.
func limit(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	return string([]rune(s)[:n])
}
