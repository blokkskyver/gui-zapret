$ErrorActionPreference = 'Stop'
Set-Location $PSScriptRoot
$env:GOMODCACHE = Join-Path $PSScriptRoot '.cache\gomod'
$env:GOCACHE = Join-Path $PSScriptRoot '.cache\gobuild'
go test ./...
if ($LASTEXITCODE -ne 0) { throw 'Tests failed' }
New-Item -ItemType Directory -Force build\bin | Out-Null
Push-Location build\windows
try {
    windres --input icon.rc --output ..\..\icon_windows_amd64.syso --output-format coff --target pe-x86-64
    if ($LASTEXITCODE -ne 0) { throw 'Icon resource build failed' }
} finally { Pop-Location }
go build -tags desktop,production -trimpath -ldflags '-H windowsgui -s -w' -o build\bin\gui-zapret.exe .
if ($LASTEXITCODE -ne 0) { throw 'Build failed' }
Write-Host 'Ready: build\bin\gui-zapret.exe'
