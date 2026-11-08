# Dokumentasi kelas3d-agent

Acuan cara kerja agent: perintah CLI, file di laptop, isi heartbeat, cara membaca jendela aktif per OS, installer, dan rilis. Kontrak API-nya mengikuti `kelas3d-api/dokumentasi.md` bagian Agent. Catatan file baru dan file yang diubah ada di bagian Riwayat perubahan, entri terbaru di paling bawah.

## Cara kerja

1. Installer (`install.ps1` atau `install.sh`) menjelaskan data yang diambil, mengunduh biner dari GitHub Releases ke folder user, lalu menambahkannya ke PATH.
2. `kelas3d login <kode>` menukar kode install pribadi dari ketua kelas jadi token laptop (`POST /api/agent/login`), menyimpannya di `config.json`, lalu mendaftarkan agent supaya jalan otomatis tiap login ke OS dan langsung menyalakannya.
3. `kelas3d run` (dijalankan service) mengambil jadwal dan `app_rules` (`GET /api/agent/config`) saat mulai dan tiap 5 menit.
4. Tiap `heartbeat_interval` detik (default 10), kalau sekarang ada di dalam jadwal `kelas` salah satu ruangan, agent membaca jendela aktif, lama idle, dan IP lokal, mencocokkannya dengan `app_rules`, lalu mengirim `POST /api/agent/heartbeat`. Di luar jadwal, agent diam dan tidak mengirim apa pun.
5. Pengumuman dan teguran di balasan heartbeat ditampilkan sebagai notifikasi sistem.
6. Kalau gagal terkirim, laporan masuk antrean dan ikut terkirim lewat `queued` di heartbeat berikutnya yang berhasil.

## Perintah CLI

| Perintah | Fungsi |
| --- | --- |
| `kelas3d login <kode> [--server <url>] [--no-service]` | Tukar kode jadi token, simpan konfigurasi, tampilkan "Laptop terhubung atas nama <nama>", lalu pasang dan nyalakan service. Flag boleh ditulis sebelum atau sesudah kode. Login ulang menghapus antrean dan laporan terakhir milik laptop lama, lalu menyalakan ulang service dengan token baru. Kalau service gagal dipasang, login tetap berhasil dan agent bisa dijalankan manual. `--no-service` untuk development (`go run . login ...`), supaya service tidak menunjuk ke biner sementara. |
| `kelas3d buka` | Minta link masuk sekali pakai (`POST /api/agent/web-link`), tampilkan link-nya, lalu buka di browser bawaan |
| `kelas3d status` | Akun, server, laptop, lokasi config dan log, jumlah antrean, dan isi heartbeat terakhir apa adanya |
| `kelas3d run` | Loop utama, biasanya dijalankan service. Berhenti rapi dengan Ctrl+C atau SIGTERM |
| `kelas3d uninstall` | Hentikan dan hapus service, hapus folder config dan cache (konfigurasi, antrean, log). File programnya dibiarkan, lokasinya ditampilkan. Laptop di server tidak ikut di-revoke |
| `kelas3d version` | Versi agent |

Kode keluar: `0` normal, `1` error, `2` perintah salah, `3` laptop sudah tidak terhubung (401 atau 403). Unit systemd tidak mengulang kalau kodenya 3.

## Alamat server

Urutan yang dipakai saat `login`: `--server`, lalu variabel `KELAS3D_SERVER`, lalu default yang ditanam saat build. Alamat itu disimpan di `config.json` dan dipakai perintah lain seterusnya. Formatnya base URL API lengkap dengan `/api`, contoh `http://localhost:8000/api`; garis miring di akhir dibuang.

Perintah install dari API (`kelas3d login <kode>`) tidak membawa `--server`, jadi biner rilis harus di-build dengan alamat server sungguhan (lihat Build dan rilis).

## File di laptop

| File | Linux | Windows | macOS | Isi |
| --- | --- | --- | --- | --- |
| `config.json` | `~/.config/kelas3d/` | `%AppData%\kelas3d\` | `~/Library/Application Support/kelas3d/` | Token, data laptop, id notifikasi terakhir. Izin `0600` |
| `queue.jsonl` | `~/.cache/kelas3d/` | `%LocalAppData%\kelas3d\` | `~/Library/Caches/kelas3d/` | Heartbeat yang belum terkirim, satu per baris, maksimal 500 |
| `last.json` | sama | sama | sama | Heartbeat terakhir untuk `kelas3d status` |
| `agent-config.json` | sama | sama | sama | Salinan `GET /api/agent/config`, dipakai kalau agent mulai saat offline |
| `agent.log`, `agent.log.1` | sama | sama | sama | Log `kelas3d run`. Lewat 1 MB, `agent.log` pindah ke `agent.log.1` (menimpa yang lama), jadi total paling besar sekitar 2 MB |

Folder dibuat dengan izin `0700`. `config.json` dan file JSON lain ditulis lewat file sementara lalu di-rename, supaya tidak rusak kalau agent mati di tengah penulisan. Token tidak pernah ditulis ke log. Saat dijalankan dari terminal, log juga muncul di stderr; di Linux, journald ikut menyimpan stderr service.

`config.json`:

```json
{
  "server": "http://localhost:8000/api",
  "token": "5|abc...",
  "device_id": 4,
  "user_name": "Budi",
  "last_announcement_id": 13,
  "last_nudge_id": null
}
```

## Service

| OS | Cara | Lokasi |
| --- | --- | --- |
| Windows | Nilai `Kelas3D` di Registry Run, isinya `"<exe>" run`. Tanpa hak admin, jalan saat user login, kelihatan di Task Manager > Startup apps. Saat login, agent juga langsung dinyalakan lepas dari terminal (`DETACHED_PROCESS`) dan agent lama dihentikan dengan `taskkill` | `HKCU\Software\Microsoft\Windows\CurrentVersion\Run` |
| Linux | Unit systemd user `kelas3d.service`: `After`, `PartOf`, dan `WantedBy=graphical-session.target`, `Restart=on-failure`, `RestartPreventExitStatus=3`. Login menjalankan `daemon-reload`, `enable`, lalu `restart` | `~/.config/systemd/user/kelas3d.service` |
| macOS | LaunchAgent `id.bengkelkoding.kelas3d` dengan `RunAtLoad`. Login menjalankan `launchctl bootout` lalu `bootstrap gui/<uid>` | `~/Library/LaunchAgents/id.bengkelkoding.kelas3d.plist` |

- Service jalan sebagai user peserta, bukan root.
- Registry Run dipilih karena trigger login di Task Scheduler (`schtasks /SC ONLOGON`) butuh hak admin. Akibatnya agent tidak di-restart otomatis kalau crash; dia baru jalan lagi di login berikutnya.
- Saat dijalankan dari Registry Run, Windows membuat jendela console untuk agent. `kelas3d run` melepasnya dengan `FreeConsole` kalau console itu cuma dipakai agent, jadi jendelanya cuma berkedip sebentar saat login. Kalau dijalankan dari terminal, console dibiarkan.
- Di Linux, unit butuh sesi grafis yang mengaktifkan `graphical-session.target` dan mengekspor `WAYLAND_DISPLAY`, `HYPRLAND_INSTANCE_SIGNATURE`, atau `DISPLAY` ke systemd user (GNOME, KDE, Hyprland lewat uwsm atau integrasi systemd-nya). Di laptop Key keduanya sudah ada. Kalau sesi tidak mengaktifkan target itu, service tidak jalan otomatis; tambahkan `exec-once = systemctl --user start kelas3d` di config Hyprland.
- LaunchAgent sengaja tanpa `KeepAlive`, karena launchd tidak bisa mengecualikan kode keluar 3 dan agent yang sudah tidak terhubung akan terus dinyalakan ulang.
- Isi ketiganya dibuat di `service.go` supaya bisa dites dari OS mana pun. Pemasangannya ada di `installService` dan `removeService` di `os_*.go`.

## Isi heartbeat

```json
{
  "activity": "ngoding",
  "app_label": "VS Code",
  "project": "toko-online",
  "file_name": "auth.ts",
  "language": "TypeScript",
  "idle_seconds": 12,
  "local_ip": "192.168.10.23",
  "sent_at": "2026-11-09T09:15:20+07:00",
  "agent_version": "0.1.0",
  "last_announcement_id": 13,
  "last_nudge_id": null,
  "queued": []
}
```

- `activity` dan `app_label` dari aturan yang cocok. Kalau tidak ada yang cocok, `activity` `lainnya` dan `app_label` nama proses tanpa `.exe` (contoh `Notepad`, `firefox`), atau `null` kalau tidak ada jendela aktif.
- `project`, `file_name`, dan `language` cuma diisi untuk `activity` `ngoding`, dari judul window (lihat bagian berikut). Selain itu `null`. Panjangnya dipotong ke 255 karakter sesuai batas API.
- `idle_seconds`: detik sejak input keyboard atau mouse terakhir.
- `local_ip`: alamat lokal koneksi UDP ke host server (`net.Dial` tanpa mengirim paket), jadi yang terambil IP interface yang dipakai ke server. Ke `localhost` hasilnya `::1` atau `127.0.0.1`. `null` kalau gagal.
- `sent_at`: ISO 8601 dengan zona waktu laptop.
- `last_announcement_id`, `last_nudge_id`: id terbesar yang sudah ditampilkan. `null` sebelum ada notifikasi; server lalu mengirim pengumuman hari ini.
- `queued`: isi antrean dengan field yang sama (tanpa `agent_version`, `last_*`, dan `queued`). Selalu dikirim sebagai array, karena API menolak `null`.

## Project, file, dan bahasa

Diambil di `title.go` dari judul window, cuma untuk hasil `ngoding`. Pola yang tidak dikenali menghasilkan `null` untuk ketiganya, jadi judul mentah tidak pernah terkirim.

| Aplikasi | Contoh judul | Hasil |
| --- | --- | --- |
| VS Code (juga Insiders, Code - OSS, VSCodium) | `● auth.ts - toko-online - Visual Studio Code` | project `toko-online`, file `auth.ts` |
| VS Code tanpa folder | `catatan.md - Visual Studio Code` | file saja |
| VS Code tanpa editor, atau tab Welcome/Settings | `toko-online - Visual Studio Code`, `Welcome - toko-online - ...` | project saja (yang kedua), atau kosong (yang pertama) |
| PyCharm dan IDE JetBrains lain | `toko-online – auth.py`, `toko-online [~/kode] – src/auth.py` | project `toko-online`, file `auth.py` |
| JupyterLab di browser | `analisis.ipynb - JupyterLab — Mozilla Firefox` | file `analisis.ipynb` |
| Jupyter Notebook klasik | `analisis - Jupyter Notebook - Google Chrome` | file `analisis.ipynb` |

- `●` (ada perubahan belum disimpan), keterangan seperti `(Working Tree)`, `(Workspace)`, dan `[SSH: lab]`, serta folder di depan nama file dibuang.
- Bagian judul tanpa ekstensi (Welcome, Settings, Untitled-1, Launcher) tidak dianggap file.
- VS Code dikenali dari akhiran judulnya. Format JetBrains (en dash) cuma dibaca kalau nama prosesnya IDE JetBrains (`pycharm`, `idea`, `webstorm`, `phpstorm`, `goland`, `clion`, `rider`, `rubymine`, `datagrip`, `studio`), supaya judul browser yang kebetulan memakai en dash tidak ikut terbaca.
- VS Code dengan profil non-default tanpa folder (`auth.ts - Work - Visual Studio Code`) terbaca dengan project `Work`. Kasus ini jarang dan dibiarkan.
- Bahasa dari ekstensi file lewat peta `languages` di `title.go` (`.ts` TypeScript, `.py` dan `.ipynb` Python, `.php` PHP, dan seterusnya). Ekstensi lain dikirim tanpa bahasa.
- Judul browser dan aplikasi chat tidak pernah dikirim, cuma kategori dan labelnya. Ini dijaga tes `TestReportNeverSendsTitles`.

## Aturan aplikasi dan jadwal

- Pencocokan `app_rules` persis seperti bagian Aturan app_rules di `kelas3d-api/dokumentasi.md`: `process` sama persis tanpa peduli huruf besar-kecil, `title` mengandung `pattern`, `os` kosong berlaku di semua OS, `priority` terbesar menang. Kalau priority sama, aturan yang lebih dulu di daftar menang (API mengurutkan prioritas tertinggi dulu). Aturan dengan `pattern` kosong diabaikan.
- Jadwal dibandingkan dalam WIB (+07:00 tetap, Indonesia tanpa DST), jadi jam laptop yang diset ke zona lain tetap benar. `day_of_week` 1 = Senin sampai 7 = Minggu. Mulai inklusif, selesai eksklusif: jadwal 08:00–16:00 berarti 08:00:00 sampai 15:59:59.
- Cuma jadwal `kelas` yang dicek. Jam istirahat ada di dalam jadwal kelas, jadi agent tetap mengirim dan server yang mengubah keadaan jadi istirahat.
- Aturan judul AI chat di seeder (vibe_coding, priority 30) juga cocok dengan jendela apa pun yang judulnya mengandung kata itu, termasuk file `chatgpt-notes.md` di VS Code. Ini urusan aturan di server, bukan agent.

## Balasan server

| Balasan | Yang dilakukan agent |
| --- | --- |
| 200 | Antrean dikosongkan, pengumuman dan teguran ditampilkan, id terakhirnya disimpan |
| Jaringan putus, 5xx, status lain | Laporan masuk antrean. Lewat 500, yang paling lama dibuang |
| 401, 403 | Berhenti, tampilkan notifikasi "Laptop ini sudah tidak terhubung, minta perintah install baru ke ketua kelas.", keluar dengan kode 3 |
| 422 | Dicatat di log sebagai bug agent, tidak masuk antrean. Antrean ikut dibuang, karena bisa jadi itu penyebabnya dan kiriman ulang cuma akan ditolak lagi |
| 429 | Laporan masuk antrean, lalu tunggu sesuai `Retry-After` (default 1 menit) |

`GET /api/agent/config` yang gagal karena jaringan dicoba lagi tiap interval; 401 atau 403 di sini diperlakukan sama dengan di heartbeat. Error yang sama tidak ditulis ulang ke log tiap 10 detik, cuma saat pesannya berubah.

## Notifikasi

| Jenis | Judul | Isi |
| --- | --- | --- |
| Pengumuman | `Pengumuman Kelas 3D` | Pesan pengumuman |
| Teguran (cuma laptop ketua kelas) | `Teguran dari koordinator: <nama peserta>` | Pesan teguran |
| Laptop tidak terhubung | `Kelas 3D` | Minta perintah install baru ke ketua kelas |

| OS | Cara |
| --- | --- |
| Windows | Toast lewat `powershell.exe` (Windows PowerShell 5.1) dengan AppID PowerShell, karena toast dengan AppID yang belum terdaftar tidak muncul. Teks lewat variabel lingkungan supaya tidak perlu di-escape, dan jendela PowerShell disembunyikan (`CREATE_NO_WINDOW`) |
| Linux | `notify-send --app-name="Kelas 3D"` |
| macOS | `osascript` `display notification`, teks lewat argv |

## Jendela aktif dan idle per OS

| Lingkungan | Jendela aktif | Idle |
| --- | --- | --- |
| Windows | `GetForegroundWindow`, `GetWindowTextW`, `GetWindowThreadProcessId`, lalu `QueryFullProcessImageNameW` untuk nama proses (contoh `Code.exe`) | `GetLastInputInfo` dibanding `GetTickCount` |
| Linux, Hyprland | `hyprctl activewindow -j` (class, title, pid), lalu nama proses dari `/proc/<pid>/comm` | Protokol Wayland `ext-idle-notify-v1` |
| Linux, X11 | `_NET_ACTIVE_WINDOW` di root window, judul dari `_NET_WM_NAME` (cadangan `WM_NAME`), pid dari `_NET_WM_PID` lalu `/proc/<pid>/comm` | Ekstensi XScreenSaver (`MsSinceUserInput`) |
| Linux, Wayland lain (GNOME, KDE) | Belum didukung | `ext-idle-notify-v1` kalau compositor-nya punya (KWin punya, Mutter tidak) |
| macOS | `osascript` ke System Events: nama proses aplikasi depan dan judul jendela depannya | `HIDIdleTime` dari `ioreg -c IOHIDSystem` (nanodetik) |

- Sesi Linux dipilih dari variabel lingkungan: `WAYLAND_DISPLAY` berarti Wayland (Hyprland kalau `HYPRLAND_INSTANCE_SIGNATURE` ada), selain itu `DISPLAY` berarti X11. X11 tidak dipakai di sesi Wayland, karena XWayland cuma melihat aplikasi X.
- Di lingkungan yang belum didukung, heartbeat tetap terkirim dengan `activity` `lainnya` dan `idle_seconds` 0, lalu peringatannya dicatat sekali di log.
- Klien `ext-idle-notify-v1` ditulis langsung di `wayland_linux.go` (beberapa pesan saja). Agent memasang notifikasi idle dengan timeout 1 detik; `idle_seconds` = waktu sejak event `idled` + 1 detik, dan kembali 0 saat `resumed`. Kalau compositor punya versi 2, yang dipakai `get_input_idle_notification`, yang cuma menghitung input keyboard dan mouse tanpa peduli idle inhibitor (video yang sedang diputar). Ini sama dengan perilaku `GetLastInputInfo`, XScreenSaver, dan `HIDIdleTime`.
- Nama di `/proc/<pid>/comm` maksimal 15 karakter (batasan kernel). VS Code di Arch (`visual-studio-code-bin`) bernama `code`, cocok dengan aturan seeder; class Hyprland-nya `com.microsoft.VSCode`, tapi yang dipakai nama proses.
- Di Windows, proses milik admin tidak bisa dibuka agent biasa: judulnya tetap terbaca, nama prosesnya kosong.
- macOS butuh dua izin untuk `kelas3d`. Automation (mengontrol System Events) diminta lewat dialog saat pertama jalan; tanpa izin ini jendela aktif tidak terbaca sama sekali. Accessibility (System Settings > Privacy & Security > Accessibility, tambahkan file `kelas3d`) dibutuhkan untuk judul jendela; tanpa izin ini yang terbaca cuma nama aplikasinya, jadi project dan file kosong. Nama proses dari System Events: VS Code `Code`, Chrome `Google Chrome`, sesuai aturan seeder.

## Installer

| Script | Perintah dari API | Tujuan biner |
| --- | --- | --- |
| `install.ps1` | `irm <INSTALL_URL>/install.ps1 \| iex; kelas3d login <kode>` | `%LocalAppData%\Programs\kelas3d\kelas3d.exe` |
| `install.sh` | `curl -fsSL <INSTALL_URL>/install.sh \| sh -s -- <kode>` | Folder di PATH yang ada di bawah `$HOME` dan bisa ditulis (`~/.local/bin` dan `~/bin` didahulukan), atau `~/.local/bin` |

Urutannya:

1. Tampilkan data yang diambil, yang tidak diambil, siapa yang bisa melihat, rencana fitur lihat layar (belum ada di versi ini), dan cara uninstall. Lanjut setelah Enter, batal dengan Ctrl+C. `install.sh` membaca Enter dari `/dev/tty`, karena stdin-nya berisi script itu sendiri.
2. Unduh biner yang cocok dari `https://github.com/Keyanffz/kelas3d-agent/releases/latest/download/`: `kelas3d-windows-amd64.exe` (Windows ARM memakainya lewat emulasi), `kelas3d-linux-amd64`, `kelas3d-darwin-arm64`, atau `kelas3d-darwin-amd64`. OS dan arsitektur lain ditolak dengan pesan jelas.
3. Taruh di folder user tanpa hak admin. Agent yang sedang jalan dihentikan dulu di Windows (file yang jalan tidak bisa ditimpa); di Linux dan macOS biner diganti lewat file sementara di folder yang sama lalu `mv`.
4. Tambahkan ke PATH. Windows: PATH user untuk terminal berikutnya dan PATH sesi ini, jadi `kelas3d login` di perintah yang sama langsung jalan. Linux dan macOS: kalau folder tujuannya belum ada di PATH, baris `export PATH="$HOME/.local/bin:$PATH"` ditambahkan ke `~/.profile`, `~/.bashrc`, dan `~/.zshrc` yang ada (dibuat sesuai `$SHELL` kalau belum ada satu pun).

Catatan:

- Di Linux dan macOS, kode diteruskan ke installer (`sh -s -- <kode>`), lalu installer menjalankan `kelas3d login` dengan path lengkap. Bentuk `| sh && kelas3d login <kode>` tidak dipakai, karena login itu jalan di shell peserta yang belum mengenal folder baru di PATH (misalnya macOS biasa, atau Arch tanpa `~/.local/bin`). Tanpa kode, installer cuma memasang biner dan, kalau foldernya belum di PATH, menampilkan perintah login dengan path lengkap.
- `KELAS3D_RELEASE` menimpa alamat unduhan, untuk mencoba sebelum rilis: `KELAS3D_RELEASE=file:///path/ke/dist sh install.sh` (spasi di path ditulis `%20`), atau `$env:KELAS3D_RELEASE` di PowerShell.
- Teks installer cuma ASCII, supaya tidak rusak saat diambil `irm` dari server yang tidak mengirim charset.

## Build dan rilis

```bash
SERVER_URL=https://kelas3d.contoh.id/api VERSION=0.1.0 ./build.sh
```

`build.sh` mem-build `windows/amd64`, `linux/amd64`, `darwin/arm64`, dan `darwin/amd64` dengan `CGO_ENABLED=0`, `-trimpath`, dan `-ldflags "-s -w -X main.serverURL=... -X main.version=..."` ke `dist/`, menyalin kedua installer, lalu menulis `SHA256SUMS`. Tiap biner sekitar 7 MB. Tanpa `SERVER_URL`, biner memakai `http://localhost:8000/api` dan script memberi peringatan.

Langkah rilis (Key yang menjalankan):

1. Commit dan push, lalu beri tag: `git tag v0.1.0 && git push origin v0.1.0`.
2. Build dengan alamat server sungguhan seperti di atas.
3. Unggah ke GitHub Releases: `gh release create v0.1.0 dist/* --title "v0.1.0" --notes "Rilis pertama agent Kelas 3D"`. Nama file di `dist/` harus tetap, karena installer mengunduh lewat `releases/latest/download/<nama>`.
4. Sajikan installer dari server API: salin `install.ps1` dan `install.sh` ke `kelas3d-api/public/`, atau set `INSTALL_URL` di `.env` API ke tempat lain yang menyajikan keduanya.
5. Coba dari laptop bersih dengan perintah install dari panel ketua kelas.

Biner Go tanpa tanda tangan kadang ditandai Windows Defender atau SmartScreen. Kalau itu terjadi saat uji di Windows, catat pesannya.

## Dependensi

| Modul | Alasan |
| --- | --- |
| `golang.org/x/sys` | API Windows tanpa cgo (`GetForegroundWindow`, `OpenProcess`, `QueryFullProcessImageName`, `LazySystemDLL`) dan paket `registry` untuk Registry Run |
| `github.com/jezek/xgb` | Protokol X11 murni Go, termasuk ekstensi XScreenSaver |

Sisanya standard library. Wayland dan macOS tidak butuh pustaka.

## Tes

```bash
go test ./...                 # logika umum + parser Wayland (di Linux)
for os in linux windows darwin; do GOOS=$os go vet ./...; done
./build.sh                    # build silang keempat target
sh -n install.sh
```

| File | Isi |
| --- | --- |
| `rules_test.go` | Pencocokan aturan (proses, judul, OS, priority, label cadangan) dan cek jadwal (batas jam, hari Minggu, zona laptop UTC) |
| `title_test.go` | Format judul VS Code, PyCharm, dan Jupyter (termasuk `●` dan judul yang tidak dikenali), peta bahasa, judul browser dan chat tidak ikut terkirim, dan pemotongan 255 karakter |
| `store_test.go` | Antrean (batas 500, simpan dan baca, kosong menghapus file), rotasi log 1 MB, dan izin `config.json` |
| `service_test.go` | Isi unit systemd (termasuk escape path), plist launchd (XML sah, tanpa KeepAlive), dan nilai Registry Run |
| `agent_test.go` | Klien API dengan `httptest` yang meniru respons `kelas3d-api`: login dan perintah `login`, config, isi heartbeat, penanganan 200/401/403/422/429/500, offline, notifikasi dan id terakhir, `buka`, `uninstall`, dan loop `run` |
| `wayland_linux_test.go` | Format string dan event global Wayland |

Tes tidak memunculkan notifikasi, membuka browser, atau memasang service: `notify` dan `openURL` diganti di `TestMain`, dan `login` dites dengan `--no-service`.

## Riwayat perubahan

### 2026-11-08 · Tahap A, login dan heartbeat dasar

File baru:

- `go.mod`, `go.sum`: module `github.com/Keyanffz/kelas3d-agent`, Go 1.27, dependensi `golang.org/x/sys` dan `github.com/jezek/xgb`.
- `.gitignore`: hasil build.
- `README.md`: ringkasan, build, dan pemakaian.
- `main.go`: perintah `login`, `run`, `status`, `version`; alamat server dan versi yang bisa ditimpa `-ldflags`.
- `api.go`: klien `POST /agent/login`, `GET /agent/config`, `POST /agent/heartbeat` dengan timeout 15 detik dan header `Accept: application/json`; bentuk data config dan heartbeat.
- `store.go`: `config.json`, antrean `queue.jsonl`, `last.json`, dan lokasinya per OS.
- `rules.go`: pencocokan `app_rules` dan cek jadwal kelas.
- `run.go`: loop `kelas3d run`, pengiriman heartbeat dan antrean, penanganan kode status, IP lokal.
- `os_linux.go`: pembaca jendela aktif dan idle untuk Hyprland dan X11, notifikasi `notify-send`.
- `wayland_linux.go`: klien Wayland minimal untuk `ext-idle-notify-v1`.
- `os_windows.go`: pembaca jendela aktif dan idle lewat Windows API, notifikasi toast.
- `rules_test.go`, `store_test.go`, `agent_test.go`, `wayland_linux_test.go`.
- `dokumentasi.md`: catatan ini.

Dari rencana tahap B, config sudah sekalian disegarkan tiap 5 menit, karena memakai kode yang sama dengan percobaan ulang saat config pertama gagal diambil.

### 2026-11-08 · Tahap B, C, dan D

Dikerjakan berurutan dalam satu jalan atas permintaan Key.

File baru:

- `title.go`: project, file, dan bahasa dari judul VS Code, IDE JetBrains, dan Jupyter; peta bahasa; pemotongan panjang.
- `service.go`: isi unit systemd, plist launchd, dan nilai Registry Run.
- `os_darwin.go`: jendela aktif lewat `osascript`, idle dari `ioreg`, notifikasi, buka URL, dan LaunchAgent.
- `install.ps1`, `install.sh`: installer tanpa hak admin.
- `build.sh`: build rilis keempat target ke `dist/`.
- `title_test.go`, `service_test.go`.

File diubah:

- `main.go`: perintah `buka` dan `uninstall`; `login` memasang service dan menerima `--no-service`; `status` menampilkan lokasi log.
- `api.go`: klien `POST /agent/web-link`.
- `run.go`: `buildReport` (laporan dari jendela aktif, project dan file cuma untuk `ngoding`), notifikasi pengumuman dan teguran beserta penyimpanan id terakhir, log ke file, `notify` dan `openURL` sebagai variabel supaya bisa diganti di tes.
- `store.go`: lokasi `agent.log` dan penulis log dengan batas 1 MB.
- `os_linux.go`: buka URL lewat `xdg-open`, pasang dan hapus unit systemd user.
- `os_windows.go`: buka URL lewat `rundll32 url.dll,FileProtocolHandler`, pasang dan hapus Registry Run, lepas console saat dijalankan dari Registry Run.
- `agent_test.go`, `store_test.go`: tes notifikasi, `buka`, `uninstall`, rotasi log; `TestMain` mengganti `notify` dan `openURL`.
- `README.md`: perintah baru, installer, dan build rilis.

### 2026-11-08 · Perintah install macOS dan Linux

`kelas3d-api` sekarang membuat perintah `curl -fsSL <INSTALL_URL>/install.sh | sh -s -- <kode>`, jadi installer yang menjalankan login. Perintah Windows tidak berubah.

File diubah:

- `install.sh`: komentar cara pakai.
- `README.md`, `dokumentasi.md`: bentuk perintah install.
