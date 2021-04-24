#!/bin/sh

mkdir -p release/JumpWay.app/Contents/{MacOS,Resources}
go generate ./webview
go build -o release/JumpWay.app/Contents/MacOS/JumpWay ./cmd/jumpway
cp icon/icon_black.png release/JumpWay.app/Contents/Resources/JumpWay.icns
./tools/icns.sh
cp tools/macos/Info.plist release/JumpWay.app/Contents/
cd release && zip JumpWay.app.zip -r JumpWay.app
rm -rf JumpWay.app