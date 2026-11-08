package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jezek/xgb"
	"github.com/jezek/xgb/screensaver"
	"github.com/jezek/xgb/xproto"
)

const osName = "linux"

var errUnsupported = errors.New("desktop ini belum didukung, cuma Hyprland dan X11")

// sensor membaca jendela aktif dan idle. Hyprland: hyprctl + ext-idle-notify-v1. X11: properti EWMH +
// XScreenSaver. Wayland lain (GNOME, KDE) belum didukung, jadi laporannya "lainnya" dengan idle 0.
type sensor struct {
	hyprland bool
	wl       *idleNotifier
	x        *xgb.Conn
	idleErr  error
}

func newSensor() *sensor {
	s := &sensor{}
	switch {
	case os.Getenv("WAYLAND_DISPLAY") != "":
		s.hyprland = os.Getenv("HYPRLAND_INSTANCE_SIGNATURE") != ""
		s.wl, s.idleErr = dialIdleNotifier(time.Second)
	case os.Getenv("DISPLAY") != "":
		x, err := xgb.NewConn()
		if err != nil {
			s.idleErr = err
			break
		}
		s.x = x
		if err := screensaver.Init(x); err != nil {
			s.idleErr = fmt.Errorf("ekstensi XScreenSaver: %w", err)
		}
	default:
		s.idleErr = errors.New("tidak ada sesi grafis (WAYLAND_DISPLAY dan DISPLAY kosong)")
	}
	return s
}

func (s *sensor) close() {
	if s.wl != nil {
		s.wl.conn.Close()
	}
	if s.x != nil {
		s.x.Close()
	}
}

func (s *sensor) window() (window, error) {
	switch {
	case s.hyprland:
		return hyprlandWindow()
	case s.x != nil:
		return x11Window(s.x)
	}
	return window{}, errUnsupported
}

func (s *sensor) idle() (time.Duration, error) {
	switch {
	case s.idleErr != nil:
		return 0, s.idleErr
	case s.wl != nil:
		return s.wl.idle()
	case s.x != nil:
		info, err := screensaver.QueryInfo(s.x, xproto.Drawable(xproto.Setup(s.x).DefaultScreen(s.x).Root)).Reply()
		if err != nil {
			return 0, err
		}
		return time.Duration(info.MsSinceUserInput) * time.Millisecond, nil
	}
	return 0, errUnsupported
}

func hyprlandWindow() (window, error) {
	out, err := exec.Command("hyprctl", "activewindow", "-j").Output()
	if err != nil {
		return window{}, fmt.Errorf("hyprctl: %w", err)
	}
	// Tanpa jendela aktif, hyprctl membalas {}.
	var w struct {
		Class string `json:"class"`
		Title string `json:"title"`
		PID   int    `json:"pid"`
	}
	if err := json.Unmarshal(out, &w); err != nil {
		return window{}, fmt.Errorf("hyprctl: %w", err)
	}
	return window{Process: procName(w.PID, w.Class), Title: w.Title}, nil
}

func x11Window(x *xgb.Conn) (window, error) {
	root := xproto.Setup(x).DefaultScreen(x).Root
	active, err := x11Prop(x, root, "_NET_ACTIVE_WINDOW")
	if err != nil || len(active) < 4 {
		return window{}, err
	}
	id := xproto.Window(xgb.Get32(active))
	if id == 0 {
		return window{}, nil
	}

	title, _ := x11Prop(x, id, "_NET_WM_NAME")
	if len(title) == 0 {
		title, _ = x11Prop(x, id, "WM_NAME")
	}
	w := window{Title: string(title)}
	if pid, _ := x11Prop(x, id, "_NET_WM_PID"); len(pid) >= 4 {
		w.Process = procName(int(xgb.Get32(pid)), "")
	}
	return w, nil
}

func x11Prop(x *xgb.Conn, win xproto.Window, name string) ([]byte, error) {
	atom, err := xproto.InternAtom(x, true, uint16(len(name)), name).Reply()
	if err != nil {
		return nil, err
	}
	prop, err := xproto.GetProperty(x, false, win, atom.Atom, xproto.GetPropertyTypeAny, 0, 1024).Reply()
	if err != nil {
		return nil, err
	}
	return prop.Value, nil
}

// procName: nama proses dari /proc/<pid>/comm (maksimal 15 karakter, batasan kernel).
func procName(pid int, fallback string) string {
	if pid > 0 {
		if b, err := os.ReadFile(fmt.Sprintf("/proc/%d/comm", pid)); err == nil {
			return strings.TrimSpace(string(b))
		}
	}
	return fallback
}

func systemNotify(title, msg string) {
	if err := exec.Command("notify-send", "--app-name=Kelas 3D", title, msg).Run(); err != nil {
		log.Printf("gagal menampilkan notifikasi: %v", err)
	}
}

func systemOpenURL(url string) error {
	return exec.Command("xdg-open", url).Start()
}

func detachConsole() {}

func unitPath() (string, error) {
	dir, err := os.UserConfigDir()
	return filepath.Join(dir, "systemd", "user", "kelas3d.service"), err
}

// installService memasang unit systemd user lalu menyalakannya ulang, supaya token baru dari login terpakai.
func installService(exe string) error {
	path, err := unitPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(systemdUnit(exe)), 0o644); err != nil {
		return err
	}
	for _, args := range [][]string{{"daemon-reload"}, {"enable", "kelas3d.service"}, {"restart", "kelas3d.service"}} {
		if err := systemctl(args...); err != nil {
			return err
		}
	}
	return nil
}

func removeService() error {
	path, err := unitPath()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}
	systemctl("disable", "--now", "kelas3d.service")
	if err := os.Remove(path); err != nil {
		return err
	}
	return systemctl("daemon-reload")
}

func systemctl(args ...string) error {
	out, err := exec.Command("systemctl", append([]string{"--user"}, args...)...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("systemctl --user %s: %v %s", strings.Join(args, " "), err, bytes.TrimSpace(out))
	}
	return nil
}
