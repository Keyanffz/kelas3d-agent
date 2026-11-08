// Agent kelas3d: melaporkan aplikasi yang dipakai, lama idle, dan IP lokal laptop peserta ke kelas3d-api.
package main

import (
	"cmp"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Bisa ditimpa saat build: -ldflags "-X main.serverURL=https://... -X main.version=0.2.0".
var (
	serverURL = "http://localhost:8000/api"
	version   = "0.1.0"
)

const usage = `Pemakaian:
  kelas3d login <kode> [--server <url>] [--no-service]
                     hubungkan laptop ini dengan kode install dari ketua kelas, lalu jalankan agent otomatis
  kelas3d buka       buka web Kelas 3D tanpa password
  kelas3d status     tampilkan akun, server, dan data terakhir yang dikirim
  kelas3d run        jalankan agent di terminal (biasanya sudah jalan otomatis)
  kelas3d uninstall  hentikan agent, hapus service, konfigurasi, antrean, dan log
`

// Kode keluar 3: laptop sudah tidak terhubung, service tidak perlu mencoba lagi.
const exitDisconnected = 3

func main() {
	if len(os.Args) < 2 {
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	var err error
	switch os.Args[1] {
	case "login":
		err = loginCmd(os.Args[2:])
	case "run":
		err = runCmd()
	case "status":
		err = statusCmd()
	case "buka":
		err = bukaCmd()
	case "uninstall":
		err = uninstallCmd()
	case "version", "--version":
		fmt.Println(version)
	default:
		fmt.Fprint(os.Stderr, usage)
		os.Exit(2)
	}

	if errors.Is(err, errDisconnected) {
		fmt.Fprintln(os.Stderr, msgDisconnected)
		os.Exit(exitDisconnected)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func loginCmd(args []string) error {
	flags := flag.NewFlagSet("login", flag.ContinueOnError)
	server := flags.String("server", "", "alamat API, contoh http://localhost:8000/api")
	noService := flags.Bool("no-service", false, "jangan pasang agent supaya jalan otomatis (untuk development)")
	// Kode boleh ditulis sebelum atau sesudah --server.
	if err := flags.Parse(args); err != nil {
		return err
	}
	code := flags.Arg(0)
	if err := flags.Parse(flags.Args()[min(1, flags.NArg()):]); err != nil {
		return err
	}
	if code == "" {
		return errors.New("kode install wajib diisi: kelas3d login <kode>")
	}

	f, err := locate()
	if err != nil {
		return err
	}
	base := strings.TrimRight(cmp.Or(*server, os.Getenv("KELAS3D_SERVER"), serverURL), "/")
	hostname, _ := os.Hostname()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	res, err := newClient(base, "").login(ctx, code, cmp.Or(hostname, "laptop"))
	if err != nil {
		var apiErr *apiError
		switch {
		case errors.As(err, &apiErr) && apiErr.Status == 422:
			return errors.New(apiErr.Message)
		case errors.As(err, &apiErr) && apiErr.Status == 429:
			return errors.New("terlalu banyak percobaan login, tunggu sebentar lalu coba lagi")
		}
		return fmt.Errorf("gagal login ke %s: %w", base, err)
	}

	cfg := config{Server: base, Token: res.Token, DeviceID: res.DeviceID, UserName: res.User.Name}
	if err := writeJSON(f.config, cfg); err != nil {
		return err
	}
	// Antrean dan laporan terakhir milik laptop yang lama.
	os.Remove(f.queue)
	os.Remove(f.last)

	fmt.Printf("Laptop terhubung atas nama %s\n", res.User.Name)
	if *noService {
		return nil
	}

	exe, err := os.Executable()
	if err == nil {
		exe, err = filepath.EvalSymlinks(exe)
	}
	if err == nil {
		err = installService(exe)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Agent belum bisa jalan otomatis: %v\nJalankan manual dengan: kelas3d run\n", err)
		return nil
	}
	fmt.Println("Agent jalan di latar belakang dan otomatis aktif tiap login ke laptop ini.")
	return nil
}

func bukaCmd() error {
	f, err := locate()
	if err != nil {
		return err
	}
	cfg, err := loadConfig(f.config)
	if err != nil {
		return err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	link, err := newClient(cfg.Server, cfg.Token).webLink(ctx)
	var apiErr *apiError
	switch {
	case disconnected(err):
		return errDisconnected
	case errors.As(err, &apiErr) && apiErr.Status == 429:
		return errors.New("terlalu sering meminta link, tunggu sebentar lalu coba lagi")
	case err != nil:
		return fmt.Errorf("gagal meminta link masuk: %w", err)
	}

	fmt.Printf("Membuka web Kelas 3D. Kalau browser tidak terbuka, salin link ini (berlaku %d menit, sekali pakai):\n%s\n", max(1, link.ExpiresIn/60), link.URL)
	if err := openURL(link.URL); err != nil {
		return fmt.Errorf("gagal membuka browser: %w", err)
	}
	return nil
}

func uninstallCmd() error {
	f, err := locate()
	if err != nil {
		return err
	}
	if err := removeService(); err != nil {
		fmt.Fprintf(os.Stderr, "Service gagal dihapus: %v\n", err)
	}
	for _, dir := range []string{filepath.Dir(f.config), filepath.Dir(f.queue)} {
		if err := os.RemoveAll(dir); err != nil {
			return err
		}
	}

	exe, _ := os.Executable()
	fmt.Printf("Agent Kelas 3D dihapus: service, konfigurasi, antrean, dan log.\n"+
		"File programnya masih ada di %s, hapus manual kalau mau.\n"+
		"Kalau laptop ini tidak dipakai lagi untuk kelas, minta ketua kelas me-revoke-nya.\n", exe)
	return nil
}

func statusCmd() error {
	f, err := locate()
	if err != nil {
		return err
	}
	cfg, err := loadConfig(f.config)
	if err != nil {
		return err
	}
	hostname, _ := os.Hostname()

	fmt.Printf("Akun        %s\n", cfg.UserName)
	fmt.Printf("Server      %s\n", cfg.Server)
	fmt.Printf("Laptop      %s (%s, device %d)\n", hostname, osName, cfg.DeviceID)
	fmt.Printf("Konfigurasi %s\n", f.config)
	fmt.Printf("Log         %s\n", f.log)
	fmt.Printf("Antrean     %d heartbeat belum terkirim\n", len(loadQueue(f.queue)))

	var last lastSent
	if err := readJSON(f.last, &last); err != nil {
		fmt.Println("\nBelum ada heartbeat yang dikirim.")
		return nil
	}
	b, _ := json.MarshalIndent(last.Report, "", "  ")
	fmt.Printf("\nHeartbeat terakhir %s, %s:\n%s\n", last.At.Local().Format("2006-01-02 15:04:05"), last.Result, b)
	if last.Queued > 0 {
		fmt.Printf("Ditambah %d heartbeat dari antrean.\n", last.Queued)
	}
	return nil
}
