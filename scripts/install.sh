#!/usr/bin/env sh
set -eu

repo="${GH_REPO:-liyown/git-spread}"
version="${VERSION:-latest}"
install_dir="${INSTALL_DIR:-$HOME/.local/bin}"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "git-spread installer requires $1" >&2
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

need curl

os="$(detect_os)"
arch="$(detect_arch)"
resolved_version="$(resolve_version)"

if [ -z "$resolved_version" ]; then
  echo "could not resolve latest git-spread version" >&2
  exit 1
fi

ext="tar.gz"
binary="git-spread"
if [ "$os" = "windows" ]; then
  ext="zip"
  binary="git-spread.exe"
  need unzip
else
  need tar
fi

asset="git-spread_${resolved_version}_${os}_${arch}.${ext}"
url="https://github.com/$repo/releases/download/$resolved_version/$asset"
tmp="${TMPDIR:-/tmp}/git-spread-install.$$"

cleanup() {
  rm -rf "$tmp"
}
trap cleanup EXIT INT TERM

mkdir -p "$tmp"
echo "Downloading $url"
curl -fsSL "$url" -o "$tmp/$asset"

if [ "$ext" = "zip" ]; then
  unzip -q "$tmp/$asset" -d "$tmp/unpack"
else
  mkdir -p "$tmp/unpack"
  tar -xzf "$tmp/$asset" -C "$tmp/unpack"
fi

found="$(find "$tmp/unpack" -type f -name "$binary" | head -n 1)"
if [ -z "$found" ]; then
  echo "archive did not contain $binary" >&2
  exit 1
fi

mkdir -p "$install_dir"
cp "$found" "$install_dir/$binary"
chmod +x "$install_dir/$binary"

echo "Installed $binary to $install_dir/$binary"
case ":$PATH:" in
  *":$install_dir:"*) ;;
  *) echo "Add $install_dir to PATH to run: git spread" ;;
esac
