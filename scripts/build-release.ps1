param([string]$Go = "go", [string]$Version = "dev")
$ErrorActionPreference = "Stop"
$root = Split-Path $PSScriptRoot -Parent
Push-Location $root
$oldOS=$env:GOOS
$oldArch=$env:GOARCH
$oldCGO=$env:CGO_ENABLED
try {
    $goCommand=(Get-Command $Go -ErrorAction Stop).Source
    & $goCommand test ./...
    if ($LASTEXITCODE -ne 0) { throw "Tests failed" }
    & $goCommand vet ./...
    if ($LASTEXITCODE -ne 0) { throw "Vet failed" }
    $out=Join-Path $root "dist/$Version"
    if (Test-Path $out) { throw "Output already exists: $out; choose a new Version" }
    New-Item -ItemType Directory -Force $out | Out-Null
    $env:CGO_ENABLED="0"
    foreach ($target in @("linux/amd64","linux/arm64","windows/amd64","windows/arm64","darwin/amd64","darwin/arm64")) {
        $env:GOOS,$env:GOARCH=$target.Split("/")
        $name="go-reverse-proxy-$Version-$($env:GOOS)-$($env:GOARCH)"
        $stage=Join-Path $out $name
        New-Item -ItemType Directory -Force $stage | Out-Null
        $binary="go-reverse-proxy"
        if ($env:GOOS -eq "windows") { $binary += ".exe" }
        & $goCommand build -trimpath -ldflags="-s -w" -o (Join-Path $stage $binary) .
        if ($LASTEXITCODE -ne 0) { throw "Build failed: $target" }
        Copy-Item README.md (Join-Path $stage "README.md")
        Copy-Item docs/binary-install.md (Join-Path $stage "INSTALL.md")
        if ($env:GOOS -eq "windows") {
            Compress-Archive -Path "$stage/*" -DestinationPath "$out/$name.zip"
        } else {
            & tar -czf "$out/$name.tar.gz" -C $stage .
            if ($LASTEXITCODE -ne 0) { throw "Archive failed: $target" }
        }
        Write-Output "Built $target"
    }
    $lines=Get-ChildItem $out -File | Sort-Object Name | ForEach-Object {
        $hash=(Get-FileHash $_.FullName -Algorithm SHA256).Hash.ToLower()
        "$hash  $($_.Name)"
    }
    $lines | Set-Content "$out/SHA256SUMS.txt" -Encoding ascii
} finally {
    $env:GOOS=$oldOS; $env:GOARCH=$oldArch; $env:CGO_ENABLED=$oldCGO
    Pop-Location
}
