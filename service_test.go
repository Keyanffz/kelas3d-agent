package main

import (
	"encoding/xml"
	"strings"
	"testing"
)

func TestSystemdUnit(t *testing.T) {
	unit := systemdUnit(`/home/budi/.local/bin/kelas 3d%`)
	for _, line := range []string{
		`ExecStart="/home/budi/.local/bin/kelas 3d%%" run`,
		"Restart=on-failure",
		"RestartPreventExitStatus=3",
		"After=graphical-session.target",
		"WantedBy=graphical-session.target",
	} {
		if !strings.Contains(unit, "\n"+line+"\n") {
			t.Errorf("unit tidak memuat %q:\n%s", line, unit)
		}
	}
}

func TestLaunchdPlist(t *testing.T) {
	plist := launchdPlist("/Users/budi & sinta/.local/bin/kelas3d")
	var doc struct {
		Keys    []string `xml:"dict>key"`
		Strings []string `xml:"dict>string"`
		Args    []string `xml:"dict>array>string"`
	}
	if err := xml.Unmarshal([]byte(plist), &doc); err != nil {
		t.Fatalf("plist bukan XML yang sah: %v\n%s", err, plist)
	}
	if strings.Join(doc.Keys, ",") != "Label,ProgramArguments,RunAtLoad" || doc.Strings[0] != launchdLabel {
		t.Errorf("isi plist: %+v", doc)
	}
	if len(doc.Args) != 2 || doc.Args[0] != "/Users/budi & sinta/.local/bin/kelas3d" || doc.Args[1] != "run" {
		t.Errorf("ProgramArguments: %q", doc.Args)
	}
	if strings.Contains(plist, "KeepAlive") {
		t.Error("KeepAlive membuat agent yang sudah tidak terhubung dinyalakan ulang terus")
	}
}

func TestWindowsRunValue(t *testing.T) {
	got := windowsRunValue(`C:\Users\Budi Santoso\AppData\Local\Programs\kelas3d\kelas3d.exe`)
	want := `"C:\Users\Budi Santoso\AppData\Local\Programs\kelas3d\kelas3d.exe" run`
	if got != want {
		t.Errorf("windowsRunValue = %s, mau %s", got, want)
	}
}
