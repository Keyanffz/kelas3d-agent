package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const osName = "windows"

// Tidak ada di golang.org/x/sys/windows, jadi dipanggil langsung dari DLL-nya.
var (
	user32                    = windows.NewLazySystemDLL("user32.dll")
	kernel32                  = windows.NewLazySystemDLL("kernel32.dll")
	procGetWindowTextW        = user32.NewProc("GetWindowTextW")
	procGetWindow             = user32.NewProc("GetWindow")
	procGetWindowLongPtrW     = user32.NewProc("GetWindowLongPtrW")
	procGetLastInputInfo      = user32.NewProc("GetLastInputInfo")
	procGetTickCount          = kernel32.NewProc("GetTickCount")
	procFreeConsole           = kernel32.NewProc("FreeConsole")
	procGetConsoleProcessList = kernel32.NewProc("GetConsoleProcessList")
)

const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`

// installService mendaftarkan agent di Registry Run (jalan saat login, tanpa hak admin), lalu menyalakannya
// sekarang juga, lepas dari terminal, karena Registry Run baru berlaku di login berikutnya.
func installService(exe string) error {
	k, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer k.Close()
	if err := k.SetStringValue("Kelas3D", windowsRunValue(exe)); err != nil {
		return err
	}

	stopOtherAgents(exe)
	cmd := exec.Command(exe, "run")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP}
	return cmd.Start()
}

func removeService() error {
	k, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err == nil {
		err = k.DeleteValue("Kelas3D")
		k.Close()
	}
	exe, _ := os.Executable()
	stopOtherAgents(exe)
	if errors.Is(err, registry.ErrNotExist) {
		return nil
	}
	return err
}

// stopOtherAgents menghentikan agent yang sedang jalan (token lama, versi lama), selain proses ini sendiri.
func stopOtherAgents(exe string) {
	cmd := exec.Command("taskkill", "/F", "/IM", filepath.Base(exe), "/FI", fmt.Sprintf("PID ne %d", os.Getpid()))
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	cmd.Run() // gagal kalau tidak ada yang jalan, tidak apa-apa
}

// detachConsole melepas jendela console yang dibuat khusus untuk agent saat dijalankan dari Registry Run,
// supaya tidak ada jendela hitam yang terbuka terus (cuma berkedip sebentar). Kalau dijalankan dari terminal,
// console dipakai bersama shell dan dibiarkan.
func detachConsole() {
	var pids [2]uint32
	if n, _, _ := procGetConsoleProcessList.Call(uintptr(unsafe.Pointer(&pids[0])), uintptr(len(pids))); n == 1 {
		procFreeConsole.Call()
	}
}

func systemOpenURL(url string) error {
	return exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}

type sensor struct{}

func newSensor() *sensor { return &sensor{} }

func (*sensor) close() {}

func (*sensor) window() (window, error) {
	hwnd := windows.GetForegroundWindow()
	if hwnd == 0 {
		return window{}, nil
	}
	return hwndWindow(hwnd)
}

// EnumWindows memanggil callback untuk tiap jendela tingkat atas, dari yang paling atas. Callback dibuat
// sekali karena jumlah callback per proses terbatas, jadi hasilnya ditampung di variabel paket; windows()
// cuma dipanggil dari loop run.
var (
	enumFound    []windows.HWND
	enumCallback = windows.NewCallback(func(hwnd windows.HWND, _ uintptr) uintptr {
		enumFound = append(enumFound, hwnd)
		return 1
	})
)

const (
	gwOwner        = 4
	gwlExStyle     = -20
	wsExToolWindow = 0x80
)

// windows: jendela aplikasi yang tampil di taskbar. Yang tersembunyi, dialog (punya owner), tool window,
// cloaked (aplikasi UWP yang tidak tampil, desktop virtual lain), desktop, dan tanpa judul dilewati.
func (*sensor) windows() ([]window, error) {
	enumFound = enumFound[:0]
	if err := windows.EnumWindows(enumCallback, nil); err != nil {
		return nil, err
	}
	shell := windows.GetShellWindow()
	list := []window{}
	for _, h := range enumFound {
		if h == shell || !windows.IsWindowVisible(h) || cloaked(h) {
			continue
		}
		if owner, _, _ := procGetWindow.Call(uintptr(h), gwOwner); owner != 0 {
			continue
		}
		idx := int32(gwlExStyle)
		if style, _, _ := procGetWindowLongPtrW.Call(uintptr(h), uintptr(idx)); style&wsExToolWindow != 0 {
			continue
		}
		if w, _ := hwndWindow(h); w.Title != "" {
			list = append(list, w)
		}
	}
	return list, nil
}

func cloaked(h windows.HWND) bool {
	var c uint32
	return windows.DwmGetWindowAttribute(h, windows.DWMWA_CLOAKED, unsafe.Pointer(&c), 4) == nil && c != 0
}

// hwndWindow: judul dan nama proses satu jendela.
func hwndWindow(hwnd windows.HWND) (window, error) {
	buf := make([]uint16, 512)
	n, _, _ := procGetWindowTextW.Call(uintptr(hwnd), uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	w := window{Title: windows.UTF16ToString(buf[:n])}

	var pid uint32
	if _, err := windows.GetWindowThreadProcessId(hwnd, &pid); err != nil {
		return w, err
	}
	// Proses milik admin tidak bisa dibuka dari agent biasa; judulnya tetap terbaca.
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		return w, err
	}
	defer windows.CloseHandle(h)

	size := uint32(len(buf))
	if err := windows.QueryFullProcessImageName(h, 0, &buf[0], &size); err != nil {
		return w, err
	}
	w.Process = filepath.Base(windows.UTF16ToString(buf[:size])) // contoh Code.exe
	return w, nil
}

func (*sensor) idle() (time.Duration, error) {
	info := struct{ cbSize, dwTime uint32 }{cbSize: 8}
	if ok, _, err := procGetLastInputInfo.Call(uintptr(unsafe.Pointer(&info))); ok == 0 {
		return 0, err
	}
	// Keduanya hitungan milidetik 32-bit sejak boot; selisih uint32 tetap benar saat angkanya berputar.
	now, _, _ := procGetTickCount.Call()
	return time.Duration(uint32(now)-info.dwTime) * time.Millisecond, nil
}

// notify menampilkan toast lewat PowerShell. Teks lewat variabel lingkungan supaya tidak perlu di-escape.
// AppID-nya milik PowerShell, karena toast dengan AppID yang belum terdaftar tidak muncul.
func systemNotify(title, msg string) {
	const script = `
[Windows.UI.Notifications.ToastNotificationManager, Windows.UI.Notifications, ContentType = WindowsRuntime] > $null
$xml = [Windows.UI.Notifications.ToastNotificationManager]::GetTemplateContent([Windows.UI.Notifications.ToastTemplateType]::ToastText02)
$text = $xml.GetElementsByTagName('text')
$text.Item(0).AppendChild($xml.CreateTextNode($env:KELAS3D_TITLE)) > $null
$text.Item(1).AppendChild($xml.CreateTextNode($env:KELAS3D_MESSAGE)) > $null
$app = '{1AC14E77-02E7-4E5D-B744-2EB1AE5198B7}\WindowsPowerShell\v1.0\powershell.exe'
[Windows.UI.Notifications.ToastNotificationManager]::CreateToastNotifier($app).Show([Windows.UI.Notifications.ToastNotification]::new($xml))
`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	cmd.Env = append(os.Environ(), "KELAS3D_TITLE="+title, "KELAS3D_MESSAGE="+msg)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: windows.CREATE_NO_WINDOW}
	if err := cmd.Run(); err != nil {
		log.Printf("gagal menampilkan notifikasi: %v", err)
	}
}
