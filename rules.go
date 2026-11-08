package main

import (
	"strings"
	"time"
)

type appRule struct {
	MatchOn  string  `json:"match_on"`
	Pattern  string  `json:"pattern"`
	OS       *string `json:"os"`
	Activity string  `json:"activity"`
	Label    string  `json:"label"`
	Priority int     `json:"priority"`
}

// window: jendela yang sedang fokus. Judulnya cuma dipakai di laptop, tidak pernah dikirim.
type window struct {
	Process string // contoh Code.exe di Windows, code di Linux
	Title   string
}

// matchApp mencocokkan jendela dengan app_rules, sesuai bagian Aturan app_rules di kelas3d-api/dokumentasi.md.
// Priority terbesar menang; kalau sama, aturan yang lebih dulu di daftar.
func matchApp(rules []appRule, w window) (activity, label string) {
	best := -1
	for i, r := range rules {
		if (r.OS != nil && *r.OS != osName) || !r.matches(w) {
			continue
		}
		if best < 0 || r.Priority > rules[best].Priority {
			best = i
		}
	}
	if best >= 0 {
		return rules[best].Activity, rules[best].Label
	}
	return "lainnya", appName(w.Process)
}

func (r appRule) matches(w window) bool {
	switch r.MatchOn {
	case "process":
		return w.Process != "" && strings.EqualFold(w.Process, r.Pattern)
	case "title":
		return r.Pattern != "" && strings.Contains(strings.ToLower(w.Title), strings.ToLower(r.Pattern))
	}
	return false
}

// appName: label untuk aplikasi yang tidak dikenali aturan mana pun, contoh Notepad.exe jadi Notepad.
func appName(process string) string {
	if n := len(process) - len(".exe"); n > 0 && strings.EqualFold(process[n:], ".exe") {
		return process[:n]
	}
	return process
}

// Jadwal di API memakai Asia/Jakarta. Indonesia tanpa DST, jadi zona tetap +07:00 cukup
// dan tidak butuh database zona waktu (yang tidak ada di Windows).
var wib = time.FixedZone("WIB", 7*60*60)

// inClass: sekarang ada di dalam jadwal "kelas" salah satu ruangan. Jadwal istirahat ada di dalam jadwal
// kelas, jadi saat istirahat agent tetap mengirim dan server yang menentukan keadaannya.
func inClass(rooms []classroom, now time.Time) bool {
	now = now.In(wib)
	day := int(now.Weekday())
	if day == 0 {
		day = 7 // API: 1 = Senin, 7 = Minggu
	}
	// "09:30:15" dibanding "HH:MM" dari API: perbandingan string sudah benar, "16:00:00" >= "16:00".
	clock := now.Format("15:04:05")
	for _, room := range rooms {
		for _, s := range room.Schedules {
			if s.Type == "kelas" && s.DayOfWeek == day && clock >= s.StartsAt && clock < s.EndsAt {
				return true
			}
		}
	}
	return false
}
