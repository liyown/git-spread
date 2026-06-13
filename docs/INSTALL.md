# Install Git Spread

Git Spread ships as a single binary named `git-spread`.

When `git-spread` is on your `PATH`, Git can run it as a subcommand:

```bash
git spread --version
```

## Supported Platforms

Release packages are built for:

| OS | Architectures | Package |
| --- | --- | --- |
| macOS | `amd64`, `arm64` | `.tar.gz` |
| Linux | `amd64`, `arm64` | `.tar.gz` |
| Windows | `amd64`, `arm64` | `.zip` |

Package names use this format:

```text
git-spread_<version>_<os>_<arch>.<ext>
```

Examples:

```text
git-spread_v0.1.0_darwin_arm64.tar.gz
git-spread_v0.1.0_linux_amd64.tar.gz
git-spread_v0.1.0_windows_amd64.zip
```

## Online Install

Install the latest release with `curl`.

macOS, Linux, or Windows Git Bash/MSYS:

```bash
curl -fsSL https://raw.githubusercontent.com/liyown/git-spread/main/scripts/install.sh | sh
```

Install a specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/liyown/git-spread/main/scripts/install.sh | VERSION=v0.1.0 sh
```

Install into a custom directory:

```bash
curl -fsSL https://raw.githubusercontent.com/liyown/git-spread/main/scripts/install.sh | INSTALL_DIR=/usr/local/bin sh
```

If your fork or release repository is different:

```bash
curl -fsSL https://raw.githubusercontent.com/liyown/git-spread/main/scripts/install.sh | GH_REPO=your-org/git-spread sh
```

The installer:

- detects OS and CPU architecture
- downloads the matching release package
- extracts `git-spread`
- installs it to `~/.local/bin` by default

It does not modify your shell profile. If `~/.local/bin` is not on `PATH`, add it yourself.

For Windows PowerShell, use the offline `.zip` package flow below.

## Offline Install

Download the package for your platform from GitHub Releases, then extract it.

macOS or Linux:

```bash
tar -xzf git-spread_v0.1.0_linux_amd64.tar.gz
cd git-spread_v0.1.0_linux_amd64
install -m 0755 git-spread ~/.local/bin/git-spread
```

Windows PowerShell:

```powershell
Expand-Archive .\git-spread_v0.1.0_windows_amd64.zip
mkdir $HOME\bin -Force
copy .\git-spread_v0.1.0_windows_amd64\git-spread.exe $HOME\bin\git-spread.exe
```

Check the install:

```bash
git spread --version
```

## Build From Source

For development or local builds:

```bash
go install ./cmd/git-spread
```

Or build a local binary:

```bash
go build -o ./bin/git-spread ./cmd/git-spread
```

## Release Packages

Official offline packages are built by GitHub Actions.

Create a release by pushing a version tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The release workflow runs:

```text
go test ./...
VERSION=<tag> ./scripts/package.sh
upload dist/* to GitHub Releases
```

You can also start the workflow manually from GitHub Actions with a version such as `v0.1.0`.

## Local Package Reproduction

The packaging script is kept in the repository so maintainers can reproduce the same packages locally before cutting a release:

```bash
VERSION=v0.1.0 ./scripts/package.sh
```

The script writes packages to `dist/` and generates `dist/checksums.txt`.

Default targets:

```text
darwin/amd64
darwin/arm64
linux/amd64
linux/arm64
windows/amd64
windows/arm64
```

Build a subset:

```bash
VERSION=v0.1.0 PLATFORMS="linux/amd64 darwin/arm64" ./scripts/package.sh
```

## Checksums

Each release should publish `checksums.txt`.

macOS:

```bash
shasum -a 256 git-spread_v0.1.0_darwin_arm64.tar.gz
```

Linux:

```bash
sha256sum git-spread_v0.1.0_linux_amd64.tar.gz
```

Compare the output with the corresponding line in `checksums.txt`.
