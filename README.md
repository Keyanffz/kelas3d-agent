# kelas3d-agent

Agent kecil di laptop peserta Kelas 3D Bengkel Koding. Selama jam kelas, agent melaporkan aplikasi yang sedang dipakai, lama idle, dan IP lokal ke `kelas3d-api` tiap 10 detik, lalu menampilkan pengumuman dan teguran dari server sebagai notifikasi. Keadaan avatar (di kelas, istirahat, keluar) ditentukan server, bukan agent.

Yang dikirim cuma kategori aktivitas, nama aplikasi, dan untuk aplikasi coding nama project, file, dan bahasanya. Judul window browser dan aplikasi chat tidak dikirim, dan agent tidak memakai kamera atau tangkapan layar.

Didukung: Windows, Linux (Hyprland dan X11), dan macOS.

## Install

Perintah install pribadi dibuat ketua kelas dari web, bentuknya:

```bash
curl -fsSL https://<server>/install.sh | sh -s -- BCDF-GHJK
```

```powershell
irm https://<server>/install.ps1 | iex; kelas3d login BCDF-GHJK
```

Setelah login, agent jalan di latar belakang dan otomatis aktif tiap login ke laptop.

## Pakai

```bash
kelas3d buka         # buka web Kelas 3D tanpa password
kelas3d status       # akun, server, dan data terakhir yang dikirim
kelas3d uninstall    # hentikan agent dan hapus semua datanya dari laptop
```

## Development

```bash
go test ./...
go run . login BCDF-GHJK --server http://localhost:8000/api --no-service
go run . run
```

Build rilis untuk semua OS ke `dist/`:

```bash
SERVER_URL=https://kelas3d.contoh.id/api VERSION=0.1.0 ./build.sh
```

Sajikan dari server API sendiri, tanpa GitHub Releases (demo di jaringan kelas, atau `localhost` untuk uji):

```bash
./sajikan.sh 192.168.1.11
```

Perintah lengkap, format file, isi heartbeat, cara baca jendela aktif per OS, dan langkah rilis ada di `dokumentasi.md`.
