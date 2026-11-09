package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

// Klien endpoint agent di kelas3d-api. Bentuk data mengikuti kelas3d-api/dokumentasi.md bagian Agent.
type client struct {
	base  string
	token string
	http  *http.Client
}

func newClient(base, token string) *client {
	return &client{base: base, token: token, http: &http.Client{Timeout: 15 * time.Second}}
}

// apiError: server membalas selain 2xx.
type apiError struct {
	Status     int
	Message    string
	RetryAfter time.Duration
}

func (e *apiError) Error() string {
	return fmt.Sprintf("server membalas %d: %s", e.Status, e.Message)
}

type loginResult struct {
	Token    string `json:"token"`
	DeviceID int64  `json:"device_id"`
	User     struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"user"`
}

type agentConfig struct {
	HeartbeatInterval int         `json:"heartbeat_interval"`
	IdleLimit         int         `json:"idle_limit"`
	Classrooms        []classroom `json:"classrooms"`
	AppRules          []appRule   `json:"app_rules"`
}

type classroom struct {
	Slug        string     `json:"slug"`
	Name        string     `json:"name"`
	NetworkCIDR *string    `json:"network_cidr"`
	Schedules   []schedule `json:"schedules"`
}

type schedule struct {
	Type      string `json:"type"`
	DayOfWeek int    `json:"day_of_week"`
	StartsAt  string `json:"starts_at"`
	EndsAt    string `json:"ends_at"`
}

// report: satu laporan. Antrean offline menyimpan bentuk yang sama, dan field null tetap dikirim.
type report struct {
	Activity string  `json:"activity"`
	AppLabel *string `json:"app_label"`
	Project  *string `json:"project"`
	FileName *string `json:"file_name"`
	Language *string `json:"language"`
	// Aplikasi lain yang jendelanya terbuka. nil (null) kalau desktop ini belum bisa membaca daftar jendela.
	OpenApps    []openApp `json:"open_apps"`
	IdleSeconds int       `json:"idle_seconds"`
	LocalIP     *string   `json:"local_ip"`
	SentAt      string    `json:"sent_at"`
}

type openApp struct {
	Activity string `json:"activity"`
	Label    string `json:"label"`
}

type heartbeat struct {
	report
	AgentVersion       string   `json:"agent_version"`
	LastAnnouncementID *int64   `json:"last_announcement_id"`
	LastNudgeID        *int64   `json:"last_nudge_id"`
	Queued             []report `json:"queued"` // jangan nil: API menolak "queued": null
}

type heartbeatReply struct {
	Announcements []struct {
		ID      int64  `json:"id"`
		Message string `json:"message"`
	} `json:"announcements"`
	Nudges []struct {
		ID      int64  `json:"id"`
		About   string `json:"about"`
		Message string `json:"message"`
	} `json:"nudges"`
}

func (c *client) login(ctx context.Context, code, hostname string) (res loginResult, err error) {
	body := map[string]string{"code": code, "hostname": hostname, "os": osName, "agent_version": version}
	err = c.do(ctx, http.MethodPost, "/agent/login", body, &res)
	return res, err
}

func (c *client) config(ctx context.Context) (res agentConfig, err error) {
	err = c.do(ctx, http.MethodGet, "/agent/config", nil, &res)
	return res, err
}

type webLink struct {
	URL       string `json:"url"`
	ExpiresIn int    `json:"expires_in"`
}

func (c *client) webLink(ctx context.Context) (res webLink, err error) {
	err = c.do(ctx, http.MethodPost, "/agent/web-link", nil, &res)
	return res, err
}

func (c *client) heartbeat(ctx context.Context, hb heartbeat) (res heartbeatReply, err error) {
	err = c.do(ctx, http.MethodPost, "/agent/heartbeat", hb, &res)
	return res, err
}

func (c *client) do(ctx context.Context, method, path string, in, out any) error {
	var body io.Reader
	if in != nil {
		b, err := json.Marshal(in)
		if err != nil {
			return err
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.base+path, body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	res, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	if res.StatusCode >= 300 {
		// Laravel menaruh pesan error di "message", termasuk validasi 422.
		var msg struct {
			Message string `json:"message"`
		}
		json.NewDecoder(io.LimitReader(res.Body, 64<<10)).Decode(&msg)
		retry, _ := strconv.Atoi(res.Header.Get("Retry-After"))
		return &apiError{Status: res.StatusCode, Message: msg.Message, RetryAfter: time.Duration(retry) * time.Second}
	}
	if out == nil {
		return nil
	}
	return json.NewDecoder(res.Body).Decode(out)
}
