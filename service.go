package main

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"strings"
)

// Isi pendaftaran service per OS. Ditaruh di luar file per OS supaya ketiganya bisa dites dari laptop mana pun;
// pemasangannya ada di installService dan removeService di os_*.go.

const launchdLabel = "id.bengkelkoding.kelas3d"

// systemdUnit: unit systemd user. Ikut graphical-session.target supaya jalan setelah sesi grafis siap dan
// WAYLAND_DISPLAY atau DISPLAY sudah ada. Kode keluar 3 (laptop tidak terhubung) tidak di-restart.
func systemdUnit(exe string) string {
	// Di ExecStart, % dan $ punya arti khusus, jadi digandakan.
	quoted := strings.NewReplacer(`\`, `\\`, `"`, `\"`, `%`, `%%`, `$`, `$$`).Replace(exe)
	return fmt.Sprintf(`[Unit]
Description=Agent Kelas 3D
After=graphical-session.target
PartOf=graphical-session.target

[Service]
ExecStart="%s" run
Restart=on-failure
RestartSec=10
RestartPreventExitStatus=3

[Install]
WantedBy=graphical-session.target
`, quoted)
}

// launchdPlist: LaunchAgent macOS, jalan saat login. Tanpa KeepAlive, karena launchd tidak bisa mengecualikan
// kode keluar 3 dan agent yang sudah tidak terhubung akan terus dinyalakan ulang.
func launchdPlist(exe string) string {
	var path bytes.Buffer
	xml.EscapeText(&path, []byte(exe))
	return fmt.Sprintf(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>Label</key>
	<string>%s</string>
	<key>ProgramArguments</key>
	<array>
		<string>%s</string>
		<string>run</string>
	</array>
	<key>RunAtLoad</key>
	<true/>
</dict>
</plist>
`, launchdLabel, path.String())
}

// windowsRunValue: isi nilai Kelas3D di HKCU\Software\Microsoft\Windows\CurrentVersion\Run.
func windowsRunValue(exe string) string {
	return `"` + exe + `" run`
}
