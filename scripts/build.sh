#!/usr/bin/env sh
set -e

mkdir -p dist

GOOS=linux GOARCH=amd64 go build -o dist/devx-linux-amd64 ./cmd/devx
GOOS=darwin GOARCH=amd64 go build -o dist/devx-darwin-amd64 ./cmd/devx
GOOS=darwin GOARCH=arm64 go build -o dist/devx-darwin-arm64 ./cmd/devx
GOOS=windows GOARCH=amd64 go build -o dist/devx-windows-amd64.exe ./cmd/devx
