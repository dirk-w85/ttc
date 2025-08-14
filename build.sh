#!/bin/sh

#go run -ldflags "-X main.curVersion=$(git describe --always --long) -X 'main.curBuild=$(date)'" main.go
echo "GIT Commit: $(git describe --always --long)"
#git describe --always --long

echo "Building for Linux/AMD64..."
podman run --rm \
-v $PWD:/app docker.io/dirkw85/dev-golang:latest \
env GOOS=linux GOARCH=amd64  go build -o ttc -ldflags "-X main.curVersion=$(git describe --always --long) -X 'main.curBuild=$(date)'" main.go 

echo "Done!"