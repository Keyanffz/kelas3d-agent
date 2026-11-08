package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// Contoh GET /api/agent/config dari kelas3d-api/dokumentasi.md.
const sampleConfig = `{
  "heartbeat_interval": 10,
  "idle_limit": 300,
  "classrooms": [
    {
      "slug": "developer",
      "name": "Developer",
      "network_cidr": "192.168.10.0/24",
      "schedules": [
        { "id": 1, "type": "kelas", "day_of_week": 1, "starts_at": "08:00", "ends_at": "16:00" },
        { "id": 2, "type": "istirahat", "day_of_week": 1, "starts_at": "12:00", "ends_at": "13:00" }
      ]
    }
  ],
  "app_rules": [
    { "match_on": "title", "pattern": "ChatGPT", "os": null, "activity": "vibe_coding", "label": "ChatGPT", "priority": 30 }
  ]
}`

func TestMain(m *testing.M) {
	// Tes tidak boleh memunculkan notifikasi atau membuka browser sungguhan.
	notify = func(title, msg string) {}
	openURL = func(url string) error { return nil }
	os.Exit(m.Run())
}

// fakeAPI meniru endpoint agent di kelas3d-api.
type fakeAPI struct {
	*httptest.Server
	config string      // isi balasan GET /agent/config
	status int         // status balasan heartbeat
	header http.Header // header tambahan balasan heartbeat
	reply  string      // isi balasan heartbeat
	onBeat func()      // dipanggil tiap heartbeat masuk

	mu      sync.Mutex
	login   map[string]string
	beats   [][]byte
	headers http.Header
}

func newFakeAPI(t *testing.T) *fakeAPI {
	api := &fakeAPI{config: sampleConfig, status: http.StatusOK, reply: `{"announcements":[],"nudges":[]}`}
	authed := func(w http.ResponseWriter, r *http.Request) bool {
		if r.Header.Get("Authorization") != "Bearer 5|abc" {
			w.WriteHeader(http.StatusUnauthorized)
			io.WriteString(w, `{"message":"Unauthenticated."}`)
			return false
		}
		return true
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/agent/login", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		json.NewDecoder(r.Body).Decode(&body)
		if body["code"] != "BCDF-GHJK" {
			w.WriteHeader(http.StatusUnprocessableEntity)
			io.WriteString(w, `{"message":"Kode tidak ditemukan, sudah dipakai, atau kedaluwarsa.","errors":{"code":["Kode tidak ditemukan, sudah dipakai, atau kedaluwarsa."]}}`)
			return
		}
		api.mu.Lock()
		api.login = body
		api.mu.Unlock()
		io.WriteString(w, `{"token":"5|abc","device_id":4,"user":{"id":7,"name":"Budi"}}`)
	})
	mux.HandleFunc("GET /api/agent/config", func(w http.ResponseWriter, r *http.Request) {
		if authed(w, r) {
			io.WriteString(w, api.config)
		}
	})
	mux.HandleFunc("POST /api/agent/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		if !authed(w, r) {
			return
		}
		b, _ := io.ReadAll(r.Body)
		api.mu.Lock()
		api.beats = append(api.beats, b)
		api.headers = r.Header.Clone()
		api.mu.Unlock()
		for k, v := range api.header {
			w.Header()[k] = v
		}
		w.WriteHeader(api.status)
		io.WriteString(w, api.reply)
		if api.onBeat != nil {
			api.onBeat()
		}
	})
	mux.HandleFunc("POST /api/agent/web-link", func(w http.ResponseWriter, r *http.Request) {
		if authed(w, r) {
			io.WriteString(w, `{"url":"http://localhost:3000/masuk?token=abc123","expires_in":120}`)
		}
	})

	api.Server = httptest.NewServer(mux)
	t.Cleanup(api.Close)
	return api
}

func (api *fakeAPI) lastBeat(t *testing.T) map[string]json.RawMessage {
	t.Helper()
	api.mu.Lock()
	defer api.mu.Unlock()
	if len(api.beats) == 0 {
		t.Fatal("belum ada heartbeat masuk")
	}
	var body map[string]json.RawMessage
	if err := json.Unmarshal(api.beats[len(api.beats)-1], &body); err != nil {
		t.Fatal(err)
	}
	return body
}

func newTestAgent(f files, api *fakeAPI, queue []report) *agent {
	return &agent{
		cfg:    config{Server: api.URL + "/api", Token: "5|abc"},
		api:    newClient(api.URL+"/api", "5|abc"),
		f:      f,
		queue:  queue,
		logged: map[string]string{},
	}
}

var sample = report{Activity: "ngoding", AppLabel: ptr("VS Code"), IdleSeconds: 12, SentAt: "2026-11-09T09:15:20+07:00"}

func TestLoginCmd(t *testing.T) {
	api := newFakeAPI(t)

	for _, args := range [][]string{
		{"BCDF-GHJK", "--server", api.URL + "/api/", "--no-service"},
		{"--no-service", "--server", api.URL + "/api", "BCDF-GHJK"},
	} {
		f := isolate(t)
		saveQueue(f.queue, []report{sample})

		if err := loginCmd(args); err != nil {
			t.Fatalf("loginCmd(%q): %v", args, err)
		}
		cfg, err := loadConfig(f.config)
		if err != nil {
			t.Fatal(err)
		}
		if cfg.Server != api.URL+"/api" || cfg.Token != "5|abc" || cfg.DeviceID != 4 || cfg.UserName != "Budi" {
			t.Errorf("config tersimpan: %+v", cfg)
		}
		if len(loadQueue(f.queue)) != 0 {
			t.Error("antrean laptop lama harus dihapus saat login")
		}
	}

	if api.login["os"] != osName || api.login["agent_version"] != version || api.login["hostname"] == "" {
		t.Errorf("body login: %v", api.login)
	}

	isolate(t)
	err := loginCmd([]string{"SALAH", "--server", api.URL + "/api", "--no-service"})
	if err == nil || err.Error() != "Kode tidak ditemukan, sudah dipakai, atau kedaluwarsa." {
		t.Errorf("kode salah: %v", err)
	}
}

func TestClientConfig(t *testing.T) {
	api := newFakeAPI(t)

	conf, err := newClient(api.URL+"/api", "5|abc").config(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if conf.HeartbeatInterval != 10 || len(conf.Classrooms[0].Schedules) != 2 || conf.AppRules[0].OS != nil || conf.AppRules[0].Priority != 30 {
		t.Errorf("config: %+v", conf)
	}

	_, err = newClient(api.URL+"/api", "token-lama").config(context.Background())
	if !disconnected(err) {
		t.Errorf("token mati harus dianggap tidak terhubung, dapat %v", err)
	}
}

func TestHeartbeatBody(t *testing.T) {
	api := newFakeAPI(t)
	a := newTestAgent(isolate(t), api, nil)

	if _, err := a.send(context.Background(), sample); err != nil {
		t.Fatal(err)
	}
	body := api.lastBeat(t)
	want := map[string]string{
		"activity":             `"ngoding"`,
		"app_label":            `"VS Code"`,
		"project":              `null`,
		"idle_seconds":         `12`,
		"local_ip":             `null`,
		"agent_version":        fmt.Sprintf("%q", version),
		"sent_at":              `"2026-11-09T09:15:20+07:00"`,
		"last_announcement_id": `null`,
		"last_nudge_id":        `null`,
		"queued":               `[]`,
	}
	for k, v := range want {
		if string(body[k]) != v {
			t.Errorf("%s = %s, mau %s", k, body[k], v)
		}
	}
	if got := api.headers.Get("Accept"); got != "application/json" {
		t.Errorf("Accept = %q", got)
	}
}

func TestSendStatus(t *testing.T) {
	tests := []struct {
		name      string
		status    int
		header    http.Header
		wantQueue int // antrean awalnya 2
		wantPause time.Duration
		wantErr   error
		result    string
	}{
		{"200: antrean ikut terkirim lalu dikosongkan", 200, nil, 0, 0, nil, "terkirim"},
		{"500: masuk antrean", 500, nil, 3, 0, nil, "masuk antrean"},
		{"422: bug di agent, antrean dibuang", 422, nil, 0, 0, nil, "ditolak server (422)"},
		{"401: token mati", 401, nil, 2, 0, errDisconnected, "ditolak, laptop tidak terhubung"},
		{"403: laptop tidak aktif", 403, nil, 2, 0, errDisconnected, "ditolak, laptop tidak terhubung"},
		{"429 dengan Retry-After", 429, http.Header{"Retry-After": {"7"}}, 3, 7 * time.Second, nil, "masuk antrean"},
		{"429 tanpa Retry-After", 429, nil, 3, time.Minute, nil, "masuk antrean"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			api := newFakeAPI(t)
			api.status, api.header = tt.status, tt.header
			f := isolate(t)
			queue := []report{sample, sample}
			saveQueue(f.queue, queue)
			a := newTestAgent(f, api, queue)

			pause, err := a.send(context.Background(), sample)
			if !errors.Is(err, tt.wantErr) || pause != tt.wantPause {
				t.Fatalf("send = %v, %v; mau %v, %v", pause, err, tt.wantPause, tt.wantErr)
			}
			if len(a.queue) != tt.wantQueue || len(loadQueue(f.queue)) != tt.wantQueue {
				t.Errorf("antrean di memori %d, di file %d; mau %d", len(a.queue), len(loadQueue(f.queue)), tt.wantQueue)
			}
			if !strings.HasPrefix(string(api.lastBeat(t)["queued"]), `[{"activity":"ngoding"`) {
				t.Errorf("queued tidak terkirim: %s", api.lastBeat(t)["queued"])
			}
			var last lastSent
			if err := readJSON(f.last, &last); err != nil || last.Result != tt.result || last.Queued != 2 {
				t.Errorf("last.json = %+v (%v)", last, err)
			}
		})
	}
}

func TestNotifications(t *testing.T) {
	var shown []string
	notify = func(title, msg string) { shown = append(shown, title+" | "+msg) }
	t.Cleanup(func() { notify = func(string, string) {} })

	api := newFakeAPI(t)
	api.reply = `{"announcements":[{"id":12,"message":"Kelas mulai"},{"id":13,"message":"Istirahat 15 menit"}],
		"nudges":[{"id":7,"about":"Budi","message":"Coba cek Budi, dari tadi idle"}]}`
	f := isolate(t)
	a := newTestAgent(f, api, nil)

	if _, err := a.send(context.Background(), sample); err != nil {
		t.Fatal(err)
	}
	want := []string{
		"Pengumuman Kelas 3D | Kelas mulai",
		"Pengumuman Kelas 3D | Istirahat 15 menit",
		"Teguran dari koordinator: Budi | Coba cek Budi, dari tadi idle",
	}
	if strings.Join(shown, "\n") != strings.Join(want, "\n") {
		t.Errorf("notifikasi:\n%s\nmau:\n%s", strings.Join(shown, "\n"), strings.Join(want, "\n"))
	}

	// Id terakhir tersimpan di config.json dan ikut di heartbeat berikutnya.
	saved, err := loadConfig(f.config)
	if err != nil || saved.LastAnnouncementID == nil || *saved.LastAnnouncementID != 13 || *saved.LastNudgeID != 7 {
		t.Fatalf("config tersimpan: %+v (%v)", saved, err)
	}
	api.reply = `{"announcements":[],"nudges":[]}`
	shown = nil
	if _, err := a.send(context.Background(), sample); err != nil {
		t.Fatal(err)
	}
	body := api.lastBeat(t)
	if string(body["last_announcement_id"]) != "13" || string(body["last_nudge_id"]) != "7" || len(shown) != 0 {
		t.Errorf("heartbeat kedua: last_announcement_id %s, last_nudge_id %s, notifikasi %v", body["last_announcement_id"], body["last_nudge_id"], shown)
	}
}

func TestBukaCmd(t *testing.T) {
	var opened string
	openURL = func(url string) error { opened = url; return nil }
	t.Cleanup(func() { openURL = func(string) error { return nil } })

	api := newFakeAPI(t)
	f := isolate(t)
	writeJSON(f.config, config{Server: api.URL + "/api", Token: "5|abc"})
	if err := bukaCmd(); err != nil {
		t.Fatal(err)
	}
	if opened != "http://localhost:3000/masuk?token=abc123" {
		t.Errorf("membuka %q", opened)
	}

	writeJSON(f.config, config{Server: api.URL + "/api", Token: "token-lama"})
	if err := bukaCmd(); !errors.Is(err, errDisconnected) {
		t.Errorf("token mati: %v", err)
	}
}

func TestUninstallCmd(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("di OS lain uninstall menghentikan agent dan mengubah registry atau launchd sungguhan")
	}
	f := isolate(t)
	writeJSON(f.config, config{Token: "5|abc"})
	saveQueue(f.queue, []report{sample})
	writeFile(f.log, []byte("log\n"))

	if err := uninstallCmd(); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{f.config, f.queue, f.log} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s masih ada", p)
		}
	}
}

func TestSendOffline(t *testing.T) {
	api := newFakeAPI(t)
	f := isolate(t)
	a := newTestAgent(f, api, nil)
	api.Close()

	if _, err := a.send(context.Background(), sample); err != nil {
		t.Fatal(err)
	}
	if len(loadQueue(f.queue)) != 1 {
		t.Error("heartbeat saat offline harus masuk antrean")
	}
}

func TestLoop(t *testing.T) {
	t.Run("token mati saat ambil config", func(t *testing.T) {
		api := newFakeAPI(t)
		a := newTestAgent(isolate(t), api, nil)
		a.api.token = "token-lama"
		s := newSensor()
		defer s.close()
		if err := a.loop(context.Background(), s); !errors.Is(err, errDisconnected) {
			t.Fatalf("loop = %v, mau errDisconnected", err)
		}
	})

	t.Run("di dalam jadwal kirim heartbeat, berhenti rapi saat dibatalkan", func(t *testing.T) {
		api := newFakeAPI(t)
		day := int(time.Now().In(wib).Weekday())
		if day == 0 {
			day = 7
		}
		api.config = fmt.Sprintf(`{"heartbeat_interval":1,"classrooms":[{"slug":"developer","schedules":[
			{"type":"kelas","day_of_week":%d,"starts_at":"00:00","ends_at":"24:00"}]}],"app_rules":[]}`, day)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		beats := 0
		api.onBeat = func() {
			if beats++; beats == 2 {
				cancel()
			}
		}

		f := isolate(t)
		a := newTestAgent(f, api, nil)
		s := newSensor()
		defer s.close()
		if err := a.loop(ctx, s); err != nil {
			t.Fatal(err)
		}
		if beats != 2 {
			t.Errorf("heartbeat masuk %d kali, mau 2", beats)
		}
		var cached agentConfig
		if err := readJSON(f.agentConfig, &cached); err != nil || cached.HeartbeatInterval != 1 {
			t.Errorf("salinan config: %+v (%v)", cached, err)
		}
	})
}
