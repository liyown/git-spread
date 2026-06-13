#!/usr/bin/env bash
set -euo pipefail

version="${VERSION:-$(git describe --tags --always --dirty 2>/dev/null || echo dev)}"
pkg_version="${version#v}"
dist_dir="${DIST_DIR:-dist}"
arches="${LINUX_ARCHES:-amd64 arm64}"
main_pkg="./cmd/git-spread"

deb_arch() {
  case "$1" in
    amd64) echo "amd64" ;;
    arm64) echo "arm64" ;;
    *) echo "unsupported deb arch: $1" >&2; exit 1 ;;
  esac
}

rpm_arch() {
  case "$1" in
    amd64) echo "x86_64" ;;
    arm64) echo "aarch64" ;;
    *) echo "unsupported rpm arch: $1" >&2; exit 1 ;;
  esac
}

build_binary() {
  local arch="$1"
  local out="$2"
  CGO_ENABLED=0 GOOS=linux GOARCH="$arch" go build \
    -trimpath \
    -ldflags "-s -w -X github.com/liyown/git-spread/internal/cli.Version=${version}" \
    -o "$out" \
    "$main_pkg"
}

build_deb() {
  local arch="$1"
  local binary="$2"
  local package_dir="${dist_dir}/work/deb/git-spread_${pkg_version}_${arch}"
  local control_dir="${package_dir}/DEBIAN"
  local install_dir="${package_dir}/usr/local/bin"

  mkdir -p "$control_dir" "$install_dir"
  install -m 0755 "$binary" "${install_dir}/git-spread"
  cat > "${control_dir}/control" <<EOF
Package: git-spread
Version: ${pkg_version}
Section: vcs
Priority: optional
Architecture: $(deb_arch "$arch")
Maintainer: Git Spread
Description: Propagate Git changes across branches.
EOF
  dpkg-deb --build --root-owner-group "$package_dir" "${dist_dir}/git-spread_${version}_linux_${arch}.deb"
}

build_archive() {
  local arch="$1"
  local binary="$2"
  local portable_dir="${dist_dir}/work/portable/git-spread_${version}_linux_${arch}"

  mkdir -p "$portable_dir"
  install -m 0755 "$binary" "${portable_dir}/git-spread"
  (cd "${dist_dir}/work/portable" && tar -czf "../../git-spread_${version}_linux_${arch}.tar.gz" "git-spread_${version}_linux_${arch}")
}

build_rpm() {
  local arch="$1"
  local binary="$2"
  local rpm_tree
  rpm_tree="$(cd "$dist_dir" && pwd -P)/work/rpm/${arch}"
  local spec="${rpm_tree}/SPECS/git-spread.spec"
  local rpm_target
  rpm_target="$(rpm_arch "$arch")"

  mkdir -p "${rpm_tree}/SPECS" "${rpm_tree}/SOURCES" "${rpm_tree}/BUILD" "${rpm_tree}/RPMS" "${rpm_tree}/SRPMS"
  install -m 0755 "$binary" "${rpm_tree}/SOURCES/git-spread"
  cat > "$spec" <<EOF
Name: git-spread
Version: ${pkg_version}
Release: 1
Summary: Propagate Git changes across branches
License: MIT

%description
Git Spread propagates branch, commit, and pull request changes across Git branches.

%prep

%build

%install
mkdir -p %{buildroot}/usr/local/bin
install -m 0755 %{_sourcedir}/git-spread %{buildroot}/usr/local/bin/git-spread

%files
/usr/local/bin/git-spread
EOF
  rpmbuild --define "_topdir ${rpm_tree}" --target "$rpm_target" -bb "$spec" >/dev/null
  cp "${rpm_tree}/RPMS/${rpm_target}/git-spread-${pkg_version}-1.${rpm_target}.rpm" "${dist_dir}/git-spread_${version}_linux_${arch}.rpm"
}

rm -rf "$dist_dir"
mkdir -p "${dist_dir}/work/bin"

for arch in $arches; do
  binary="${dist_dir}/work/bin/git-spread-linux-${arch}"
  echo "Building Linux installers for ${arch}"
  build_binary "$arch" "$binary"
  build_archive "$arch" "$binary"
  build_deb "$arch" "$binary"
  build_rpm "$arch" "$binary"
done

rm -rf "${dist_dir}/work"
echo "Created Linux installers and portable binaries in ${dist_dir}"
