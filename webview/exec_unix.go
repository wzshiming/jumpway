package webview

import (
	_ "embed"
)

//go:generate go build -o webview ./cmd/webview

//go:embed webview
var view []byte
