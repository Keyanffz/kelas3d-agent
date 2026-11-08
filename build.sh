#!/usr/bin/env bash
# Build rilis semua target ke dist/, siap diunggah ke GitHub Releases.
#   SERVER_URL=https://kelas3d.contoh.id/api VERSION=0.1.0 ./build.sh
set -euo pipefail
cd "$(dirname "$0")"

server=${SERVER_URL:-http://localhost:8000/api}
flags="-s -w -X main.serverURL=$server"
if [[ -n ${VERSION:-} ]]; then
	flags+=" -X main.version=$VERSION"
fi
if [[ -z ${SERVER_URL:-} ]]; then
	echo "Peringatan: SERVER_URL kosong, biner memakai $server dan cuma bisa dipakai di laptop ini." >&2
fi

rm -rf dist
mkdir dist
for target in windows/amd64 linux/amd64 darwin/arm64 darwin/amd64; do
	os=${target%/*}
	arch=${target#*/}
	out=dist/kelas3d-$os-$arch
	[[ $os == windows ]] && out+=.exe
	CGO_ENABLED=0 GOOS=$os GOARCH=$arch go build -trimpath -ldflags "$flags" -o "$out" .
done
cp install.sh install.ps1 dist/
(cd dist && sha256sum kelas3d-* >SHA256SUMS)
ls -lh dist
