package gitspread

import (
	"os"
	"strings"
	"testing"
)

func TestReleasePublishesPortableBinaries(t *testing.T) {
	files := map[string]string{
		"release workflow": readFile(t, ".github/workflows/release.yml"),
		"README":           readFile(t, "README.md"),
		"macOS package":    readFile(t, "scripts/package-macos.sh"),
		"Linux package":    readFile(t, "scripts/package-linux.sh"),
		"Windows package":  readFile(t, "scripts/package-windows.ps1"),
	}

	required := map[string][]string{
		"release workflow": {
			"portable-binaries",
			"dist/*.tar.gz",
			"dist/*.zip",
		},
		"README": {
			"Portable executables",
			"git-spread_<version>_darwin_universal.tar.gz",
			"git-spread_<version>_linux_amd64.tar.gz",
			"git-spread_<version>_linux_arm64.tar.gz",
			"git-spread_<version>_windows_amd64.zip",
			"git-spread_<version>_windows_arm64.zip",
		},
		"macOS package": {
			"git-spread_${version}_darwin_universal.tar.gz",
		},
		"Linux package": {
			"git-spread_${version}_linux_${arch}.tar.gz",
		},
		"Windows package": {
			"git-spread_${Version}_windows_${Arch}.zip",
		},
	}

	for name, wants := range required {
		content := files[name]
		for _, want := range wants {
			if !strings.Contains(content, want) {
				t.Fatalf("%s does not include %q", name, want)
			}
		}
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(data)
}
