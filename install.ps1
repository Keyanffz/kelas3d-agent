# Installer agent Kelas 3D untuk Windows, tanpa hak admin.
#   irm https://<server>/install.ps1 | iex; kelas3d login <kode>
# Dijalankan lewat iex di sesi PowerShell peserta, jadi isinya dibungkus supaya variabelnya tidak tertinggal.
& {
    $ErrorActionPreference = 'Stop'
    $ProgressPreference = 'SilentlyContinue' # progress bar Invoke-WebRequest membuat unduhan sangat lambat
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12

    # KELAS3D_RELEASE untuk mencoba biner dari tempat lain sebelum dirilis.
    $release = if ($env:KELAS3D_RELEASE) { $env:KELAS3D_RELEASE } else { 'https://github.com/Keyanffz/kelas3d-agent/releases/latest/download' }
    $dir = Join-Path $env:LOCALAPPDATA 'Programs\kelas3d'
    $exe = Join-Path $dir 'kelas3d.exe'

    Write-Host @'
Agent Kelas 3D akan dipasang di laptop ini.

Yang dikirim ke server tiap 10 detik, cuma selama jam kelas:
  - kategori dan nama aplikasi yang sedang dipakai, contoh "ngoding, VS Code"
  - kategori dan nama aplikasi lain yang jendelanya sedang terbuka, tanpa judul jendelanya
  - untuk aplikasi coding (VS Code, Cursor, Antigravity, PyCharm, Jupyter, dan sejenisnya):
    nama project, file, dan bahasanya
  - lama tidak ada ketikan atau gerakan mouse
  - IP lokal laptop, untuk tahu kamu ada di ruangan mana
Tidak dikirim: judul jendela browser dan aplikasi chat, isi layar, isi file, dan kamera.

Yang bisa melihat:
  - teman sekelas: avatarmu, kategori aktivitas, dan nama aplikasinya
  - ketua kelas dan koordinator: ditambah nama project, file, bahasa, daftar aplikasi yang terbuka,
    dan rekap waktunya
Rencana ke depan: koordinator bisa melihat layarmu secara langsung (tanpa rekaman) lewat ketua kelas.
Fitur itu belum ada di versi ini dan akan diberitahukan sebelum dipakai.

Di luar jam kelas agent diam. Hapus kapan saja dengan: kelas3d uninstall

'@
    Read-Host 'Tekan Enter untuk lanjut, atau Ctrl+C untuk batal' | Out-Null

    # Windows ARM menjalankan biner amd64 lewat emulasi, jadi cukup satu biner.
    New-Item -ItemType Directory -Force -Path $dir | Out-Null
    # File yang sedang jalan tidak bisa ditimpa di Windows.
    Get-Process -Name kelas3d -ErrorAction SilentlyContinue | Stop-Process -Force
    Write-Host 'Mengunduh kelas3d-windows-amd64.exe ...'
    Invoke-WebRequest -UseBasicParsing -Uri "$release/kelas3d-windows-amd64.exe" -OutFile "$exe.tmp"
    Move-Item -Force "$exe.tmp" $exe
    Write-Host "Terpasang di $exe"

    # PATH user untuk terminal berikutnya, dan PATH sesi ini supaya "kelas3d login" langsung jalan.
    $userPath = [Environment]::GetEnvironmentVariable('Path', 'User')
    if (($userPath -split ';') -notcontains $dir) {
        $newPath = if ($userPath) { $userPath.TrimEnd(';') + ";$dir" } else { $dir }
        [Environment]::SetEnvironmentVariable('Path', $newPath, 'User')
    }
    if (($env:Path -split ';') -notcontains $dir) {
        $env:Path = "$env:Path;$dir"
    }
}
