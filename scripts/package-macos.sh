#!/usr/bin/env bash
set -euo pipefail

version="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
pkg_version="${version#v}"
dist_dir="${DIST_DIR:-dist}"
main_pkg="./cmd/git-spread"

if ! command -v pkgbuild >/dev/null 2>&1; then
  echo "pkgbuild is required to create macOS installers" >&2
  exit 1
fi

if ! command -v lipo >/dev/null 2>&1; then
  echo "lipo is required to create a universal macOS binary" >&2
  exit 1
fi

rm -rf "$dist_dir"
mkdir -p "${dist_dir}/work/root/usr/local/bin" "${dist_dir}/work/bin"

for arch in amd64 arm64; do
  echo "Building darwin/${arch}"
  CGO_ENABLED=0 GOOS=darwin GOARCH="$arch" go build \
    -trimpath \
    -ldflags "-s -w -X github.com/liyown/git-spread/internal/cli.Version=${version}" \
    -o "${dist_dir}/work/bin/git-spread-${arch}" \
    "$main_pkg"
done

lipo -create \
  "${dist_dir}/work/bin/git-spread-amd64" \
  "${dist_dir}/work/bin/git-spread-arm64" \
  -output "${dist_dir}/work/root/usr/local/bin/git-spread"

pkgbuild \
  --root "${dist_dir}/work/root" \
  --identifier "com.liyown.git-spread" \
  --version "$pkg_version" \
  --install-location "/" \
  "${dist_dir}/git-spread_${version}_darwin_universal.pkg"

rm -rf "${dist_dir}/work"
echo "Created macOS installer in ${dist_dir}"
