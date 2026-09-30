#!/usr/bin/env bash
set -euo pipefail

version=${1:-}
if [[ ! $version =~ ^v[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  printf 'Usage: %s vX.Y.Z\n' "$0" >&2
  exit 2
fi

repo_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
output_dir="$repo_dir/dist"
stage_dir=$(mktemp -d)
trap 'rm -rf "$stage_dir"' EXIT

cd "$repo_dir"
npm --prefix frontend run build
mkdir -p "$output_dir" "$stage_dir/linux" "$stage_dir/windows"

go build -trimpath -tags 'production,webkit2_41' -ldflags '-s -w' -o "$stage_dir/linux/UltimateAnime" .
GOOS=windows GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -tags production -ldflags '-H=windowsgui -s -w' -o "$stage_dir/windows/UltimateAnime.exe" .

for platform in linux windows; do
  cp config.example.json followed.example.json README-用户使用.md CHANGELOG.md LICENSE "$stage_dir/$platform/"
  archive="$output_dir/UltimateAnime-${version}-${platform}-amd64.zip"
  (cd "$stage_dir/$platform" && zip -q -r "$archive" .)
  printf 'Created %s\n' "$archive"
done

(cd "$output_dir" && sha256sum "UltimateAnime-${version}-linux-amd64.zip" "UltimateAnime-${version}-windows-amd64.zip" > "UltimateAnime-${version}-SHA256SUMS.txt")
