package main

import (
	"bytes"
	"testing"
)

func TestParseGlobal(t *testing.T) {
	// Panjang nama interface 7, 8, dan 20 karakter: padding 0, 3, dan 3 byte setelah NUL.
	for _, iface := range []string{"wl_seat", "wl_shell", "ext_idle_notifier_v1"} {
		body := bytes.Join([][]byte{u32(42), wlString(iface), u32(2)}, nil)
		if len(body)%4 != 0 {
			t.Errorf("%s: panjang %d bukan kelipatan 4", iface, len(body))
		}
		name, got, ver, ok := parseGlobal(body)
		if !ok || name != 42 || got != iface || ver != 2 {
			t.Errorf("parseGlobal(%s) = %d, %q, %d, %v", iface, name, got, ver, ok)
		}
		if _, _, _, ok := parseGlobal(body[:len(body)-4]); ok {
			t.Errorf("%s: pesan terpotong harus ditolak", iface)
		}
	}
}
