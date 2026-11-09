#!/usr/bin/env bash
# Sajikan agent dari server kelas3d-api sendiri, tanpa GitHub Releases: build dengan alamat server itu,
# lalu taruh biner dan installer di folder public API. Dipakai untuk demo di jaringan kelas dan uji lokal.
#   ./sajikan.sh 192.168.1.11      # API di http://192.168.1.11:8000
#   ./sajikan.sh localhost         # uji di laptop ini
set -euo pipefail
cd "$(dirname "$0")"

case ${1:-} in
'')
	echo "Pakai: ./sajikan.sh <IP atau host API, sama dengan host di APP_URL .env API>" >&2
	exit 2
	;;
http://* | https://*) base=${1%/} ;;
*) base=http://$1:8000 ;;
esac
public=${KELAS3D_PUBLIC:-../kelas3d-api/public}
if [[ ! -d $public ]]; then
	echo "Folder $public tidak ada. Set KELAS3D_PUBLIC ke folder public kelas3d-api." >&2
	exit 1
fi

SERVER_URL=$base/api ./build.sh >/dev/null

rm -rf "$public/agent"
mkdir "$public/agent"
cp dist/kelas3d-* dist/SHA256SUMS "$public/agent/"

# Installer mengunduh biner dari server ini, bukan dari GitHub Releases.
github=https://github.com/Keyanffz/kelas3d-agent/releases/latest/download
for f in install.sh install.ps1; do
	if ! grep -qF "$github" "$f"; then
		echo "URL rilis di $f berubah, sesuaikan sajikan.sh." >&2
		exit 1
	fi
	sed "s|$github|$base/agent|" "$f" >"$public/$f"
done

echo "Agent untuk $base/api disajikan dari $base:"
echo "  $base/install.sh, $base/install.ps1, $base/agent/"
echo "APP_URL (atau INSTALL_URL) di .env API harus $base, supaya perintah dari panel ketua menunjuk ke sini."
