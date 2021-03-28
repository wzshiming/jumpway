@echo off

mkdir release
go generate ./webview
go build -ldflags="-H=windowsgui" -o release/JumpWay.exe ./cmd/jumpway
