#!/bin/sh

set -e

mkdir -p release
for ARCH in amd64 arm64; do
	CGO_ENABLED=0 GOOS=linux GOARCH=$ARCH go build -o "release/JumpWay_linux_$ARCH" ./cmd/jumpway
done
