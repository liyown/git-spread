#!/usr/bin/env bash
set -euo pipefail

version="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
dist_dir="${DIST_DIR:-dist}"
package_prefix="git-spread"
main_pkg="./cmd/git-spread"
platforms="${PLATFORMS:-darwin/amd64 darwin/arm64 linux/amd64 linux/arm64 windows/amd64 windows/arm64}"

checksum_tool() {
  if command -v sha256sum >/dev/null 2>&1; then
    echo "sha256sum"
  elif command -v shasum >/dev/null 2>&1; then
    echo "shasum -a 256"
  else
    echo "sha256sum or shasum is required" >&2
    exit 1
  fi
}

build_one() {
  local goos="$1"
  local goarch="$2"
  local ext=""
  local archive_ext="tar.gz"
  local package_name="${package_prefix}_${version}_${goos}_${goarch}"
  local build_dir="${dist_dir}/build/${package_name}"

  if [[ "$goos" == "windows" ]]; then
    ext=".exe"
    archive_ext="zip"
  fi

  mkdir -p "$build_dir"
  echo "Building ${goos}/${goarch}"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build \
    -trimpath \
    -ldflags "-s -w -X github.com/liyown/git-spread/internal/cli.Version=${version}" \
    -o "${build_dir}/git-spread${ext}" \
    "$main_pkg"

  cp README.md "$build_dir/README.md"

  if [[ "$archive_ext" == "zip" ]]; then
    (cd "${dist_dir}/build" && zip -qr "../${package_name}.zip" "$package_name")
  else
    (cd "${dist_dir}/build" && tar -czf "../${package_name}.tar.gz" "$package_name")
  fi
}

rm -rf "$dist_dir"
mkdir -p "$dist_dir/build"

for platform in $platforms; do
  goos="${platform%/*}"
  goarch="${platform#*/}"
  build_one "$goos" "$goarch"
done

(
  cd "$dist_dir"
  checksum_cmd="$(checksum_tool)"
  # shellcheck disable=SC2086
  $checksum_cmd git-spread_"$version"_* > checksums.txt
)

rm -rf "$dist_dir/build"

echo "Created packages in $dist_dir"
