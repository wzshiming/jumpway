package webview

import (
	_ "embed"
)

//go:generate go build -o webview.exe ./cmd/webview

//go:embed webview.exe
var view []byte
