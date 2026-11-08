package main

import (
	"cmp"
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/url"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var errDisconnected = errors.New("laptop tidak terhubung")

const msgDisconnected = "Laptop ini sudah tidak terhubung, minta perintah install baru ke ketua kelas."

const configEvery = 5 * time.Minute

// Diganti di tes supaya tidak memunculkan notifikasi atau membuka browser sungguhan.
var (
	notify  = systemNotify
	openURL = systemOpenURL
)

type agent struct {
	cfg    config
	api    *client
	f      files
	queue  []report
	logged map[string]string
}

func runCmd() error {
	f, err := locate()
	if err != nil {
		return err
	}
	cfg, err := loadConfig(f.config)
	if err != nil {
		return err
	}

	detachConsole()
	if lf, err := openLog(f.log); err != nil {
		log.Printf("log tidak bisa ditulis ke file: %v", err)
	} else {
		// File dulu: setelah console dilepas di Windows, menulis ke stderr gagal dan MultiWriter berhenti di situ.
		log.SetOutput(io.MultiWriter(lf, os.Stderr))
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	s := newSensor()
	defer s.close()

	a := &agent{cfg: cfg, api: newClient(cfg.Server, cfg.Token), f: f, queue: loadQueue(f.queue), logged: map[string]string{}}
	log.Printf("agent %s jalan atas nama %s, server %s", version, cfg.UserName, cfg.Server)
	err = a.loop(ctx, s)
	if errors.Is(err, errDisconnected) {
		notify("Kelas 3D", msgDisconnected)
	}
	return err
}

func (a *agent) loop(ctx context.Context, s *sensor) error {
	var conf *agentConfig
	var fetchedAt time.Time

	for {
		if time.Since(fetchedAt) >= configEvery {
			c, err := a.api.config(ctx)
			if disconnected(err) {
				return errDisconnected
			}
			if err == nil {
				conf, fetchedAt = &c, time.Now()
				if err := writeJSON(a.f.agentConfig, c); err != nil {
					log.Printf("gagal menyimpan salinan config: %v", err)
				}
			} else if conf == nil {
				// Mulai saat offline: pakai salinan config terakhir supaya heartbeat tetap masuk antrean.
				var cached agentConfig
				if readJSON(a.f.agentConfig, &cached) == nil {
					conf = &cached
				}
			}
			a.logChange("config", errMsg("gagal mengambil config", err))
		}

		wait := 10 * time.Second
		if conf != nil {
			wait = time.Duration(cmp.Or(conf.HeartbeatInterval, 10)) * time.Second
			now := time.Now()
			if inClass(conf.Classrooms, now) {
				a.logChange("jadwal", "di dalam jadwal kelas, heartbeat dikirim")
				pause, err := a.send(ctx, a.collect(s, conf, now))
				if err != nil {
					return err
				}
				wait = max(wait, pause)
			} else {
				a.logChange("jadwal", "di luar jadwal kelas, agent diam")
			}
		}

		select {
		case <-ctx.Done():
			log.Print("agent berhenti")
			return nil
		case <-time.After(wait):
		}
	}
}

func (a *agent) collect(s *sensor, conf *agentConfig, now time.Time) report {
	w, err := s.window()
	a.logChange("window", errMsg("gagal membaca jendela aktif", err))
	idle, err := s.idle()
	a.logChange("idle", errMsg("gagal membaca waktu idle", err))
	return buildReport(conf.AppRules, w, idle, localIP(a.cfg.Server), now)
}

// buildReport menyusun laporan dari jendela aktif. Judul jendela tidak pernah masuk laporan: project, file,
// dan bahasa cuma diambil untuk aktivitas ngoding, dan cuma kalau pola judulnya dikenali.
func buildReport(rules []appRule, w window, idle time.Duration, ip string, now time.Time) report {
	activity, label := matchApp(rules, w)
	r := report{
		Activity:    activity,
		AppLabel:    nullable(limit(label, 255)),
		IdleSeconds: int(idle / time.Second),
		LocalIP:     nullable(ip),
		SentAt:      now.Format(time.RFC3339),
	}
	if activity == "ngoding" {
		project, file := codeDetail(w)
		r.Project, r.FileName, r.Language = nullable(limit(project, 255)), nullable(limit(file, 255)), nullable(language(file))
	}
	return r
}

// send mengirim satu heartbeat beserta antrean. pause > 0 berarti server minta menunggu (429).
// Error cuma dikembalikan kalau laptop sudah tidak terhubung (401, 403).
func (a *agent) send(ctx context.Context, r report) (pause time.Duration, err error) {
	hb := heartbeat{
		report:             r,
		AgentVersion:       version,
		LastAnnouncementID: a.cfg.LastAnnouncementID,
		LastNudgeID:        a.cfg.LastNudgeID,
		Queued:             append([]report{}, a.queue...),
	}
	reply, err := a.api.heartbeat(ctx, hb)

	var apiErr *apiError
	errors.As(err, &apiErr)
	result := "terkirim"
	switch {
	case err == nil:
		if len(hb.Queued) > 0 {
			log.Printf("tersambung lagi, %d heartbeat dari antrean ikut terkirim", len(hb.Queued))
		}
		a.logChange("heartbeat", "")
		a.setQueue(nil)
		a.showReply(reply)
	case disconnected(err):
		result, err = "ditolak, laptop tidak terhubung", errDisconnected
	case apiErr != nil && apiErr.Status == 422:
		// Bentuk laporan salah berarti bug di agent. Antrean ikut dibuang, karena bisa jadi itu penyebabnya
		// dan mengirim ulang cuma akan ditolak lagi.
		log.Printf("heartbeat ditolak server (422), antrean dibuang: %s", apiErr.Message)
		result, err = "ditolak server (422)", nil
		a.setQueue(nil)
	default:
		// Jaringan putus, server error, atau 429: simpan untuk dikirim ulang.
		if apiErr != nil && apiErr.Status == 429 {
			pause = cmp.Or(apiErr.RetryAfter, time.Minute)
		}
		a.logChange("heartbeat", errMsg("heartbeat gagal, disimpan di antrean", err))
		result, err = "masuk antrean", nil
		a.setQueue(pushQueue(a.queue, r))
	}

	if err := writeJSON(a.f.last, lastSent{At: time.Now(), Result: result, Report: r, Queued: len(hb.Queued)}); err != nil {
		log.Printf("gagal menyimpan heartbeat terakhir: %v", err)
	}
	return pause, err
}

// showReply menampilkan pengumuman dan teguran sebagai notifikasi, lalu menyimpan id terakhirnya supaya
// notifikasi yang sama tidak muncul lagi. Teguran cuma datang ke laptop ketua kelas.
func (a *agent) showReply(reply heartbeatReply) {
	if len(reply.Announcements) == 0 && len(reply.Nudges) == 0 {
		return
	}
	for _, n := range reply.Announcements {
		notify("Pengumuman Kelas 3D", n.Message)
		a.cfg.LastAnnouncementID = ptrMax(a.cfg.LastAnnouncementID, n.ID)
	}
	for _, n := range reply.Nudges {
		notify("Teguran dari koordinator: "+n.About, n.Message)
		a.cfg.LastNudgeID = ptrMax(a.cfg.LastNudgeID, n.ID)
	}
	if err := writeJSON(a.f.config, a.cfg); err != nil {
		log.Printf("gagal menyimpan id notifikasi terakhir: %v", err)
	}
}

func ptrMax(p *int64, v int64) *int64 {
	if p != nil && *p >= v {
		return p
	}
	return &v
}

func (a *agent) setQueue(q []report) {
	a.queue = q
	if err := saveQueue(a.f.queue, q); err != nil {
		log.Printf("gagal menyimpan antrean: %v", err)
	}
}

// logChange mencatat pesan cuma kalau berbeda dari pesan terakhir dengan kunci yang sama,
// supaya log tidak terisi hal yang sama tiap 10 detik. Pesan kosong menandai keadaan sudah normal.
func (a *agent) logChange(key, msg string) {
	if a.logged[key] == msg {
		return
	}
	a.logged[key] = msg
	if msg != "" {
		log.Print(msg)
	}
}

func errMsg(prefix string, err error) string {
	if err == nil {
		return ""
	}
	return prefix + ": " + err.Error()
}

func disconnected(err error) bool {
	var apiErr *apiError
	return errors.As(err, &apiErr) && (apiErr.Status == 401 || apiErr.Status == 403)
}

func nullable(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// localIP: IP interface yang dipakai ke server, bukan IP pertama yang kebetulan ada.
// Dial UDP cuma memilih rute, tidak mengirim paket apa pun.
func localIP(server string) string {
	u, err := url.Parse(server)
	if err != nil || u.Hostname() == "" {
		return ""
	}
	conn, err := net.Dial("udp", net.JoinHostPort(u.Hostname(), cmp.Or(u.Port(), "80")))
	if err != nil {
		return ""
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}
