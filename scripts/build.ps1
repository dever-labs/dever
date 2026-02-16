$ErrorActionPreference = "Stop"
New-Item -ItemType Directory -Force -Path dist | Out-Null

$env:GOOS = "linux"; $env:GOARCH = "amd64"; go build -o dist/devx-linux-amd64 ./cmd/devx
$env:GOOS = "darwin"; $env:GOARCH = "amd64"; go build -o dist/devx-darwin-amd64 ./cmd/devx
$env:GOOS = "darwin"; $env:GOARCH = "arm64"; go build -o dist/devx-darwin-arm64 ./cmd/devx
$env:GOOS = "windows"; $env:GOARCH = "amd64"; go build -o dist/devx-windows-amd64.exe ./cmd/devx

Remove-Item Env:GOOS
Remove-Item Env:GOARCH
