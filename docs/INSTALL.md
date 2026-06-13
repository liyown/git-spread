# Install

## Online

Latest:

```bash
curl -fsSL https://raw.githubusercontent.com/liyown/git-spread/main/scripts/install.sh | sh
```

Specific version:

```bash
curl -fsSL https://raw.githubusercontent.com/liyown/git-spread/main/scripts/install.sh | VERSION=v0.1.0 sh
```

Custom directory:

```bash
curl -fsSL https://raw.githubusercontent.com/liyown/git-spread/main/scripts/install.sh | INSTALL_DIR=/usr/local/bin sh
```

## Offline Artifacts

Download the artifact for your platform from GitHub Releases.

| Platform | Artifact |
| --- | --- |
| macOS Intel | `git-spread_<version>_darwin_amd64.tar.gz` |
| macOS Apple Silicon | `git-spread_<version>_darwin_arm64.tar.gz` |
| Linux x64 | `git-spread_<version>_linux_amd64.tar.gz` |
| Linux ARM64 | `git-spread_<version>_linux_arm64.tar.gz` |
| Windows x64 | `git-spread_<version>_windows_amd64.zip` |
| Windows ARM64 | `git-spread_<version>_windows_arm64.zip` |
| Checksums | `checksums.txt` |

macOS / Linux:

```bash
tar -xzf git-spread_v0.1.0_linux_amd64.tar.gz
install -m 0755 git-spread_v0.1.0_linux_amd64/git-spread ~/.local/bin/git-spread
```

Windows PowerShell:

```powershell
Expand-Archive .\git-spread_v0.1.0_windows_amd64.zip
mkdir $HOME\bin -Force
copy .\git-spread_v0.1.0_windows_amd64\git-spread.exe $HOME\bin\git-spread.exe
```

Verify:

```bash
git spread --version
```

## Release Action

Push a tag:

```bash
git tag v0.1.0
git push origin v0.1.0
```

GitHub Actions publishes:

```text
git-spread_v0.1.0_darwin_amd64.tar.gz
git-spread_v0.1.0_darwin_arm64.tar.gz
git-spread_v0.1.0_linux_amd64.tar.gz
git-spread_v0.1.0_linux_arm64.tar.gz
git-spread_v0.1.0_windows_amd64.zip
git-spread_v0.1.0_windows_arm64.zip
checksums.txt
```
