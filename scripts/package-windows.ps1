param(
    [string]$Version = $env:VERSION,
    [string]$DistDir = $(if ($env:DIST_DIR) { $env:DIST_DIR } else { "dist" }),
    [string]$WindowsArches = $(if ($env:WINDOWS_ARCHES) { $env:WINDOWS_ARCHES } else { "amd64 arm64" })
)

$ErrorActionPreference = "Stop"

if (-not $Version) {
    $Version = (git describe --tags --always --dirty 2>$null)
    if (-not $Version) {
        $Version = "dev"
    }
}

$PackageVersion = $Version.TrimStart("v")
$MainPackage = "./cmd/git-spread"
$UpgradeCode = "7f5a7ce4-9d5f-4f1c-9262-bef4d764a3cb"

if (-not (Get-Command wix -ErrorAction SilentlyContinue)) {
    throw "wix is required. Install with: dotnet tool install --global wix"
}

Remove-Item -Recurse -Force $DistDir -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path "$DistDir/work" | Out-Null

foreach ($Arch in $WindowsArches.Split(" ", [System.StringSplitOptions]::RemoveEmptyEntries)) {
    Write-Host "Building windows/$Arch"
    $BuildDir = Join-Path $DistDir "work/$Arch"
    New-Item -ItemType Directory -Force -Path $BuildDir | Out-Null
    $ExePath = Join-Path $BuildDir "git-spread.exe"

    $env:CGO_ENABLED = "0"
    $env:GOOS = "windows"
    $env:GOARCH = $Arch
    go build `
        -trimpath `
        -ldflags "-s -w -X github.com/liyown/git-spread/internal/cli.Version=$Version" `
        -o $ExePath `
        $MainPackage

    $PortableDir = Join-Path $BuildDir "portable/git-spread_${Version}_windows_${Arch}"
    New-Item -ItemType Directory -Force -Path $PortableDir | Out-Null
    Copy-Item -Path $ExePath -Destination (Join-Path $PortableDir "git-spread.exe")
    $ZipPath = Join-Path $DistDir "git-spread_${Version}_windows_${Arch}.zip"
    Compress-Archive -Path $PortableDir -DestinationPath $ZipPath -Force

    $WixArch = if ($Arch -eq "amd64") { "x64" } elseif ($Arch -eq "arm64") { "arm64" } else { throw "unsupported Windows arch: $Arch" }
    $WxsPath = Join-Path $BuildDir "git-spread.wxs"
    $MsiPath = Join-Path $DistDir "git-spread_${Version}_windows_${Arch}.msi"
    $ExeSource = [System.Security.SecurityElement]::Escape((Resolve-Path $ExePath).Path)

    @"
<Wix xmlns="http://wixtoolset.org/schemas/v4/wxs">
  <Package Name="Git Spread" Manufacturer="Git Spread" Version="$PackageVersion" UpgradeCode="$UpgradeCode" Scope="perMachine">
    <MajorUpgrade DowngradeErrorMessage="A newer version of Git Spread is already installed." />
    <MediaTemplate EmbedCab="yes" />
    <StandardDirectory Id="ProgramFilesFolder">
      <Directory Id="INSTALLFOLDER" Name="Git Spread">
        <Component Id="GitSpreadExe" Guid="*">
          <File Id="GitSpreadExeFile" Source="$ExeSource" KeyPath="yes" />
          <Environment Id="GitSpreadPath" Name="PATH" Value="[INSTALLFOLDER]" Permanent="no" Part="last" Action="set" System="yes" />
        </Component>
      </Directory>
    </StandardDirectory>
    <Feature Id="MainFeature" Title="Git Spread" Level="1">
      <ComponentRef Id="GitSpreadExe" />
    </Feature>
  </Package>
</Wix>
"@ | Set-Content -Path $WxsPath -Encoding UTF8

    wix build -acceptEula wix7 -arch $WixArch -o $MsiPath $WxsPath
}

Remove-Item -Recurse -Force (Join-Path $DistDir "work")
Write-Host "Created Windows installers and portable binaries in $DistDir"
