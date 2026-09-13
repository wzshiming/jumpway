@echo off

mkdir release
go run github.com/tc-hib/go-winres@v0.3.3 make --in=tools/windows/winres.json --out=cmd/jumpway/main --arch=amd64,arm64 || exit /b 1
set CGO_ENABLED=0
set GOARCH=amd64
go build -ldflags="-H=windowsgui" -o release/JumpWay_windows_amd64.exe ./cmd/jumpway || exit /b 1
set GOARCH=arm64
go build -ldflags="-H=windowsgui" -o release/JumpWay_windows_arm64.exe ./cmd/jumpway || exit /b 1
