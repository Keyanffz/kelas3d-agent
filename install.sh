#!/bin/sh
# Installer agent Kelas 3D untuk Linux dan macOS, tanpa hak admin.
#   curl -fsSL https://<server>/install.sh | sh -s -- <kode>
# Kode diteruskan ke "kelas3d login" dengan path lengkap. Tanpa kode, cuma memasang biner.
set -eu

# KELAS3D_RELEASE untuk mencoba biner dari tempat lain, contoh file:///home/key/kelas3d-agent/dist.
release=${KELAS3D_RELEASE:-https://github.com/Keyanffz/kelas3d-agent/releases/latest/download}

cat <<'EOF'
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

EOF
printf 'Tekan Enter untuk lanjut, atau Ctrl+C untuk batal. '
# stdin berisi script ini sendiri (curl | sh), jadi jawaban dibaca dari terminal.
read -r _ 2>/dev/null </dev/tty || echo

case "$(uname -s)" in
Linux) os=linux ;;
Darwin) os=darwin ;;
*) echo "OS $(uname -s) belum didukung." >&2; exit 1 ;;
esac
case "$(uname -m)" in
x86_64 | amd64) arch=amd64 ;;
arm64 | aarch64) arch=arm64 ;;
*) echo "Arsitektur $(uname -m) belum didukung." >&2; exit 1 ;;
esac
if [ "$os" = linux ] && [ "$arch" != amd64 ]; then
	echo "Linux $arch belum didukung, cuma amd64." >&2
	exit 1
fi

# Folder tujuan: yang sudah ada di PATH dan bisa ditulis tanpa admin, supaya "kelas3d login" langsung jalan
# di terminal ini. ~/.local/bin dan ~/bin didahulukan. Daftarnya lewat variabel, karena IFS cuma memecah
# hasil ekspansi: titik dua yang ditulis langsung di "for" membuat tiga kandidat pertama jadi satu.
dir=
in_path=1
kandidat=$HOME/.local/bin:$HOME/bin:$PATH
IFS=:
for d in $kandidat; do
	case ":$PATH:" in *":$d:"*) ;; *) continue ;; esac
	case "$d" in "$HOME"/*) ;; *) continue ;; esac
	if [ -d "$d" ] && [ -w "$d" ]; then
		dir=$d
		break
	fi
done
unset IFS
if [ -z "$dir" ]; then
	dir=$HOME/.local/bin
	case ":$PATH:" in *":$dir:"*) ;; *) in_path= ;; esac
fi
mkdir -p "$dir"

echo "Mengunduh kelas3d-$os-$arch ..."
# File sementara di folder yang sama, supaya mv mengganti biner lama (yang mungkin sedang jalan) sekaligus.
tmp=$(mktemp "$dir/.kelas3d.XXXXXX")
trap 'rm -f "$tmp"' EXIT
curl -fsSL "$release/kelas3d-$os-$arch" -o "$tmp"
chmod 755 "$tmp"
mv "$tmp" "$dir/kelas3d"
echo "Terpasang di $dir/kelas3d"

if [ -z "$in_path" ]; then
	# Untuk terminal berikutnya. Terminal yang sekarang belum mengenal folder ini.
	line='export PATH="$HOME/.local/bin:$PATH"'
	case "${SHELL:-}" in */zsh) touch "$HOME/.zshrc" ;; *) touch "$HOME/.profile" ;; esac
	for rc in "$HOME/.profile" "$HOME/.bashrc" "$HOME/.zshrc"; do
		if [ -f "$rc" ] && ! grep -qF "$line" "$rc"; then
			printf '\n%s\n' "$line" >>"$rc"
		fi
	done
	echo "Folder $dir ditambahkan ke PATH untuk terminal baru."
	if [ $# -eq 0 ]; then
		echo "Di terminal ini, jalankan dengan path lengkap: $dir/kelas3d login <kode>"
	fi
fi

if [ $# -gt 0 ]; then
	"$dir/kelas3d" login "$@"
fi
