@echo off

mkdir release
go get github.com/tc-hib/go-winres
go run github.com/tc-hib/go-winres make --in=tools/windows/winres.json --out=cmd/jumpway/main
go build -ldflags="-H=windowsgui" -o release/JumpWay.exe ./cmd/jumpway
