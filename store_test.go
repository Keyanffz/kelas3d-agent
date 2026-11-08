package main

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"
)

// isolate mengarahkan folder config dan cache ke folder sementara.
func isolate(t *testing.T) files {
	t.Helper()
	dir := t.TempDir()
	for _, key := range []string{"HOME", "XDG_CONFIG_HOME", "XDG_CACHE_HOME", "AppData", "LocalAppData"} {
		t.Setenv(key, dir)
	}
	f, err := locate()
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestPushQueueDropsOldest(t *testing.T) {
	var q []report
	for i := range maxQueue + 1 {
		q = pushQueue(q, report{IdleSeconds: i})
	}
	if len(q) != maxQueue || q[0].IdleSeconds != 1 || q[maxQueue-1].IdleSeconds != maxQueue {
		t.Fatalf("antrean: panjang %d, pertama %d, terakhir %d", len(q), q[0].IdleSeconds, q[len(q)-1].IdleSeconds)
	}
}

func TestQueueFile(t *testing.T) {
	path := isolate(t).queue
	q := []report{
		{Activity: "ngoding", AppLabel: ptr("VS Code"), IdleSeconds: 3, LocalIP: ptr("192.168.10.23"), SentAt: "2026-11-09T09:00:00+07:00"},
		{Activity: "lainnya", SentAt: "2026-11-09T09:00:10+07:00"},
	}
	if err := saveQueue(path, q); err != nil {
		t.Fatal(err)
	}
	if got := loadQueue(path); !reflect.DeepEqual(got, q) {
		t.Fatalf("loadQueue = %+v, mau %+v", got, q)
	}

	if err := saveQueue(path, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("antrean kosong harus menghapus file")
	}
	if err := saveQueue(path, nil); err != nil {
		t.Fatalf("menghapus antrean yang sudah tidak ada: %v", err)
	}
}

func TestLogRotation(t *testing.T) {
	path := isolate(t).log
	chunk := make([]byte, maxLog/3+1)
	for range 3 {
		l, err := openLog(path) // dibuka ulang, seperti agent yang dinyalakan lagi
		if err != nil {
			t.Fatal(err)
		}
		l.Write(chunk)
		l.f.Close()
	}
	cur, _ := os.Stat(path)
	old, err := os.Stat(path + ".1")
	if err != nil || cur.Size() != int64(len(chunk)) || old.Size() != 2*int64(len(chunk)) {
		t.Fatalf("setelah lewat 1 MB: agent.log %d byte, agent.log.1 %v (%v)", cur.Size(), old, err)
	}
}

func TestConfigFilePrivate(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("izin file Unix")
	}
	path := isolate(t).config
	if err := writeJSON(path, config{Token: "5|abc"}); err != nil {
		t.Fatal(err)
	}
	for p, want := range map[string]os.FileMode{path: 0o600, filepath.Dir(path): 0o700} {
		info, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != want {
			t.Errorf("izin %s = %v, mau %v", p, info.Mode().Perm(), want)
		}
	}
}
