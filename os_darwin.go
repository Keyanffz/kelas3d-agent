package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const osName = "macos"

// Aplikasi depan dan judul jendelanya lewat System Events. Judul butuh izin Accessibility untuk kelas3d;
// tanpa izin itu yang terbaca cuma nama aplikasinya.
const frontScript = `tell application "System Events"
	set p to first application process whose frontmost is true
	set t to ""
	try
		set t to name of front window of p
	end try
	return (name of p) & linefeed & t
end tell`

// Aplikasi yang punya jendela (bukan proses latar). Judulnya tidak diambil, cukup nama aplikasinya.
const appsScript = `tell application "System Events"
	set AppleScript's text item delimiters to linefeed
	return (name of every application process whose background only is false) as text
end tell`

var hidIdle = regexp.MustCompile(`"HIDIdleTime" = (\d+)`)

type sensor struct{}

func newSensor() *sensor { return &sensor{} }

func (*sensor) close() {}

func (*sensor) window() (window, error) {
	out, err := exec.Command("osascript", "-e", frontScript).Output()
	if err != nil {
		return window{}, fmt.Errorf("osascript (cek izin Automation untuk System Events): %w", err)
	}
	name, title, _ := strings.Cut(strings.TrimRight(string(out), "\n"), "\n")
	return window{Process: name, Title: title}, nil
}

// windows: aplikasi yang sedang terbuka, tanpa urutan pemakaian dan tanpa judul, jadi aturan judul (ChatGPT di
// browser) tidak berlaku di sini.
func (*sensor) windows() ([]window, error) {
	out, err := exec.Command("osascript", "-e", appsScript).Output()
	if err != nil {
		return nil, fmt.Errorf("osascript: %w", err)
	}
	list := []window{}
	for name := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		if name != "" {
			list = append(list, window{Process: name})
		}
	}
	return list, nil
}

// idle: HIDIdleTime dari IOHIDSystem, dalam nanodetik.
func (*sensor) idle() (time.Duration, error) {
	out, err := exec.Command("ioreg", "-c", "IOHIDSystem").Output()
	if err != nil {
		return 0, err
	}
	m := hidIdle.FindSubmatch(out)
	if m == nil {
		return 0, errors.New("HIDIdleTime tidak ditemukan di ioreg")
	}
	ns, err := strconv.ParseInt(string(m[1]), 10, 64)
	return time.Duration(ns), err
}

// systemNotify: teks lewat argv osascript supaya tidak perlu di-escape.
func systemNotify(title, msg string) {
	script := []string{"-e", "on run argv", "-e", "display notification (item 2 of argv) with title (item 1 of argv)", "-e", "end run"}
	if err := exec.Command("osascript", append(script, title, msg)...).Run(); err != nil {
		log.Printf("gagal menampilkan notifikasi: %v", err)
	}
}

func systemOpenURL(url string) error {
	return exec.Command("open", url).Start()
}

func detachConsole() {}

func plistPath() (string, error) {
	home, err := os.UserHomeDir()
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"), err
}

// installService memasang LaunchAgent. RunAtLoad langsung menyalakannya; bootout dulu supaya agent yang
// sedang jalan dengan token lama berhenti.
func installService(exe string) error {
	path, err := plistPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(launchdPlist(exe)), 0o644); err != nil {
		return err
	}
	domain := fmt.Sprintf("gui/%d", os.Getuid())
	exec.Command("launchctl", "bootout", domain+"/"+launchdLabel).Run() // gagal kalau belum terpasang
	if out, err := exec.Command("launchctl", "bootstrap", domain, path).CombinedOutput(); err != nil {
		return fmt.Errorf("launchctl bootstrap: %v %s", err, bytes.TrimSpace(out))
	}
	return nil
}

func removeService() error {
	path, err := plistPath()
	if err != nil {
		return err
	}
	exec.Command("launchctl", "bootout", fmt.Sprintf("gui/%d/%s", os.Getuid(), launchdLabel)).Run()
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}
