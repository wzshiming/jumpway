package webview

import (
	_ "embed"
)

//go:generate go build -ldflags="-H=windowsgui" -o webview.exe ./cmd/webview

//go:embed webview.exe
var view []byte
