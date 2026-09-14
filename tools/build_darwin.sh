#!/bin/sh

set -e

mkdir -p release/JumpWay.app/Contents/MacOS release/JumpWay.app/Contents/Resources
cp icon/icon_black.png release/JumpWay.app/Contents/Resources/JumpWay.icns
./tools/icns.sh
cp tools/macos/Info.plist release/JumpWay.app/Contents/

BUILD=$(mktemp -d)
trap 'rm -rf "$BUILD"' EXIT
# Pin CGO_ENABLED: Go's default flips with the host arch, which would make the two slices differ.
CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -o "$BUILD/arm64" ./cmd/jumpway
CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -o "$BUILD/amd64" ./cmd/jumpway
lipo -create -output "$BUILD/universal" "$BUILD/arm64" "$BUILD/amd64"

for ARCH in universal arm64 amd64; do
	cp "$BUILD/$ARCH" release/JumpWay.app/Contents/MacOS/JumpWay
	(cd release && zip -qr "JumpWay_darwin_$ARCH.app.zip" JumpWay.app)
done

rm -rf release/JumpWay.app
