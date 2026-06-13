#!/usr/bin/env sh
set -eu

repo="${GH_REPO:-liyown/git-spread}"
version="${VERSION:-latest}"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "git-spread installer requires $1" >&2
    exit 1
  fi
}

run_privileged() {
  if [ "$(id -u 2>/dev/null || echo 1)" = "0" ]; then
    "$@"
  elif command -v sudo >/dev/null 2>&1; then
    sudo "$@"
  else
    echo "installation requires root privileges; re-run with sudo" >&2
    exit 1
  fi
}

detect_os() {
  os="$(uname -s | tr '[:upper:]' '[:lower:]')"
  case "$os" in
    linux) echo "linux" ;;
    darwin) echo "darwin" ;;
    mingw*|msys*|cygwin*) echo "windows" ;;
    *) echo "unsupported OS: $os" >&2; exit 1 ;;
  esac
}

detect_arch() {
  arch="$(uname -m)"
  case "$arch" in
    x86_64|amd64) echo "amd64" ;;
    arm64|aarch64) echo "arm64" ;;
    *) echo "unsupported architecture: $arch" >&2; exit 1 ;;
  esac
}

resolve_version() {
  if [ "$version" != "latest" ]; then
    echo "$version"
    return
  fi
  curl -fsSL "https://api.github.com/repos/$repo/releases/latest" |
    sed -n 's/.*"tag_name"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p' |
    head -n 1
}

linux_ext() {
  if command -v dpkg >/dev/null 2>&1; then
    echo "deb"
    return
  fi
  if command -v rpm >/dev/null 2>&1; then
    echo "rpm"
    return
  fi
  echo "linux install requires dpkg or rpm" >&2
  exit 1
}

install_artifact() {
  os="$1"
  artifact="$2"
  case "$os" in
    darwin)
      run_privileged installer -pkg "$artifact" -target /
      ;;
    linux)
      case "$artifact" in
        *.deb) run_privileged dpkg -i "$artifact" ;;
        *.rpm) run_privileged rpm -Uvh "$artifact" ;;
      esac
      ;;
    windows)
      msiexec.exe /i "$(cygpath -w "$artifact" 2>/dev/null || printf '%s' "$artifact")"
      ;;
  esac
}

need curl

os="$(detect_os)"
arch="$(detect_arch)"
resolved_version="$(resolve_version)"

if [ -z "$resolved_version" ]; then
  echo "could not resolve latest git-spread version" >&2
  exit 1
fi

case "$os" in
  darwin)
    asset="git-spread_${resolved_version}_darwin_universal.pkg"
    ;;
  linux)
    ext="$(linux_ext)"
    asset="git-spread_${resolved_version}_linux_${arch}.${ext}"
    ;;
  windows)
    asset="git-spread_${resolved_version}_windows_${arch}.msi"
    ;;
esac

url="https://github.com/$repo/releases/download/$resolved_version/$asset"
tmp="${TMPDIR:-/tmp}/git-spread-install.$$"

cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT INT TERM

mkdir -p "$tmp"
echo "Downloading $url"
curl -fsSL "$url" -o "$tmp/$asset"

install_artifact "$os" "$tmp/$asset"
echo "Installed git-spread ${resolved_version}"
