package main

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Klien Wayland minimal untuk ext-idle-notify-v1: compositor mengirim "idled" setelah user diam selama
// timeout, dan "resumed" begitu ada input lagi. Cuma butuh beberapa pesan, jadi ditulis langsung tanpa pustaka.
//
// Format pesan: id objek (u32), ukuran<<16 | opcode (u32), lalu argumen rata 4 byte dalam urutan byte mesin.
// String: panjang termasuk NUL (u32), isi, NUL, lalu padding.

var ne = binary.NativeEndian

// Id objek dipilih klien, mulai dari 2 (1 = wl_display).
const (
	wlDisplay = 1 + iota
	wlRegistry
	wlSync
	wlSeat
	idleNotifierID
	idleNotificationID
)

type idleNotifier struct {
	conn    net.Conn
	timeout time.Duration

	mu    sync.Mutex
	since time.Time // waktu event idled; nol berarti user sedang aktif
	err   error
}

func dialIdleNotifier(timeout time.Duration) (*idleNotifier, error) {
	path := os.Getenv("WAYLAND_DISPLAY")
	if !filepath.IsAbs(path) {
		path = filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), path)
	}
	conn, err := net.Dial("unix", path)
	if err != nil {
		return nil, err
	}
	n := &idleNotifier{conn: conn, timeout: timeout}
	if err := n.setup(); err != nil {
		conn.Close()
		return nil, fmt.Errorf("wayland: %w", err)
	}
	go n.listen()
	return n, nil
}

func (n *idleNotifier) idle() (time.Duration, error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if n.err != nil || n.since.IsZero() {
		return 0, n.err
	}
	return time.Since(n.since) + n.timeout, nil
}

func (n *idleNotifier) setup() error {
	// get_registry lalu sync: semua global sudah diumumkan sebelum callback sync datang.
	n.send(wlDisplay, 1, u32(wlRegistry))
	n.send(wlDisplay, 0, u32(wlSync))

	var seat, notifier, notifierVersion uint32
	for done := false; !done; {
		obj, op, body, err := n.read()
		switch {
		case err != nil:
			return err
		case obj == wlDisplay && op == 0:
			return displayError(body)
		case obj == wlSync && op == 0:
			done = true
		case obj == wlRegistry && op == 0:
			name, iface, ver, ok := parseGlobal(body)
			switch {
			case !ok:
				return errors.New("pesan global rusak")
			case iface == "wl_seat" && seat == 0:
				seat = name
			case iface == "ext_idle_notifier_v1":
				notifier, notifierVersion = name, ver
			}
		}
	}
	if seat == 0 || notifier == 0 {
		return errors.New("compositor tidak mendukung ext-idle-notify-v1")
	}

	// wl_registry.bind(name, interface, version, new_id)
	ver := min(notifierVersion, 2)
	n.send(wlRegistry, 0, u32(seat), wlString("wl_seat"), u32(1), u32(wlSeat))
	n.send(wlRegistry, 0, u32(notifier), wlString("ext_idle_notifier_v1"), u32(ver), u32(idleNotifierID))

	// Versi 2 punya get_input_idle_notification (opcode 2) yang cuma menghitung input keyboard dan mouse,
	// tanpa peduli idle inhibitor seperti video yang sedang diputar. Sama dengan GetLastInputInfo di Windows.
	op := uint16(1)
	if ver >= 2 {
		op = 2
	}
	return n.send(idleNotifierID, op, u32(idleNotificationID), u32(uint32(n.timeout.Milliseconds())), u32(wlSeat))
}

func (n *idleNotifier) listen() {
	for {
		obj, op, body, err := n.read()
		n.mu.Lock()
		switch {
		case err != nil:
			n.err = fmt.Errorf("wayland: %w", err)
		case obj == wlDisplay && op == 0:
			n.err = fmt.Errorf("wayland: %w", displayError(body))
		case obj == idleNotificationID && op == 0: // idled
			n.since = time.Now()
		case obj == idleNotificationID && op == 1: // resumed
			n.since = time.Time{}
		}
		stop := n.err != nil
		n.mu.Unlock()
		if stop {
			return
		}
	}
}

func (n *idleNotifier) read() (obj uint32, op uint16, body []byte, err error) {
	var head [8]byte
	if _, err = io.ReadFull(n.conn, head[:]); err != nil {
		return
	}
	obj, word := ne.Uint32(head[0:]), ne.Uint32(head[4:])
	if word>>16 < 8 {
		return 0, 0, nil, errors.New("ukuran pesan rusak")
	}
	body = make([]byte, word>>16-8)
	_, err = io.ReadFull(n.conn, body)
	return obj, uint16(word), body, err
}

func (n *idleNotifier) send(obj uint32, op uint16, args ...[]byte) error {
	body := bytes.Join(args, nil)
	msg := ne.AppendUint32(nil, obj)
	msg = ne.AppendUint32(msg, uint32(8+len(body))<<16|uint32(op))
	_, err := n.conn.Write(append(msg, body...))
	return err
}

func u32(v uint32) []byte { return ne.AppendUint32(nil, v) }

func wlString(s string) []byte {
	b := ne.AppendUint32(nil, uint32(len(s)+1))
	b = append(b, s...)
	return append(b, make([]byte, 4-len(s)%4)...) // NUL plus padding ke kelipatan 4
}

// parseGlobal membaca event wl_registry.global(name: uint, interface: string, version: uint).
func parseGlobal(b []byte) (name uint32, iface string, version uint32, ok bool) {
	if len(b) < 12 {
		return
	}
	size := int(ne.Uint32(b[4:]))
	end := 8 + (size+3)&^3
	if size < 1 || end+4 > len(b) {
		return
	}
	return ne.Uint32(b), string(b[8 : 8+size-1]), ne.Uint32(b[end:]), true
}

// wl_display.error(object_id: object, code: uint, message: string)
func displayError(b []byte) error {
	if len(b) >= 12 {
		if size := int(ne.Uint32(b[8:])); size >= 1 && 12+size <= len(b) {
			return fmt.Errorf("error compositor %d: %s", ne.Uint32(b[4:]), b[12:12+size-1])
		}
	}
	return errors.New("error compositor")
}
