package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"time"
)

// config.json: token laptop dan id notifikasi terakhir. Izin 0600 karena berisi token.
type config struct {
	Server             string `json:"server"`
	Token              string `json:"token"`
	DeviceID           int64  `json:"device_id"`
	UserName           string `json:"user_name"`
	LastAnnouncementID *int64 `json:"last_announcement_id"`
	LastNudgeID        *int64 `json:"last_nudge_id"`
}

// lastSent: heartbeat terakhir untuk ditampilkan kelas3d status.
type lastSent struct {
	At     time.Time `json:"at"`
	Result string    `json:"result"`
	Report report    `json:"report"`
	Queued int       `json:"queued"`
}

type files struct {
	config      string // UserConfigDir/kelas3d/config.json
	queue       string // UserCacheDir/kelas3d/queue.jsonl, heartbeat yang belum terkirim
	last        string // UserCacheDir/kelas3d/last.json
	agentConfig string // UserCacheDir/kelas3d/agent-config.json, salinan GET /agent/config untuk mulai saat offline
	log         string // UserCacheDir/kelas3d/agent.log
}

func locate() (files, error) {
	conf, err := os.UserConfigDir()
	if err != nil {
		return files{}, err
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return files{}, err
	}
	conf, cache = filepath.Join(conf, "kelas3d"), filepath.Join(cache, "kelas3d")
	return files{
		config:      filepath.Join(conf, "config.json"),
		queue:       filepath.Join(cache, "queue.jsonl"),
		last:        filepath.Join(cache, "last.json"),
		agentConfig: filepath.Join(cache, "agent-config.json"),
		log:         filepath.Join(cache, "agent.log"),
	}, nil
}

const maxLog = 1 << 20

// logFile: log agent di file. Lewat 1 MB, isinya dipindah ke agent.log.1 (menimpa yang lama) lalu mulai
// dari kosong, jadi total log paling besar sekitar 2 MB.
type logFile struct {
	path string
	f    *os.File
	size int64
}

func openLog(path string) (*logFile, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}
	return &logFile{path: path, f: f, size: info.Size()}, nil
}

func (l *logFile) Write(p []byte) (int, error) {
	if l.size+int64(len(p)) > maxLog {
		l.f.Close() // Windows tidak bisa me-rename file yang masih terbuka
		os.Rename(l.path, l.path+".1")
		f, err := os.OpenFile(l.path, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
		if err != nil {
			return 0, err
		}
		l.f, l.size = f, 0
	}
	n, err := l.f.Write(p)
	l.size += int64(n)
	return n, err
}

func loadConfig(path string) (config, error) {
	var cfg config
	if err := readJSON(path, &cfg); err != nil || cfg.Token == "" {
		return cfg, errors.New("laptop ini belum terhubung, jalankan: kelas3d login <kode dari ketua kelas>")
	}
	return cfg, nil
}

func readJSON(path string, v any) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(b, v)
}

// writeJSON menulis lewat file sementara lalu rename, supaya config tidak rusak kalau agent mati di tengah jalan.
func writeJSON(path string, v any) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFile(path, b)
}

func writeFile(path string, b []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

const maxQueue = 500

// pushQueue menambah laporan ke antrean. Lewat 500, yang paling lama dibuang.
func pushQueue(q []report, r report) []report {
	q = append(q, r)
	return q[max(0, len(q)-maxQueue):]
}

func loadQueue(path string) []report {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()

	var q []report
	scan := bufio.NewScanner(f)
	for scan.Scan() {
		var r report
		if json.Unmarshal(scan.Bytes(), &r) == nil {
			q = append(q, r)
		}
	}
	return q
}

// saveQueue menulis satu laporan per baris. Antrean kosong berarti file dihapus.
func saveQueue(path string, q []report) error {
	if len(q) == 0 {
		if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	}
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, r := range q {
		if err := enc.Encode(r); err != nil {
			return err
		}
	}
	return writeFile(path, buf.Bytes())
}
