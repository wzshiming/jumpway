package main

import (
	"os"
	"strconv"

	"github.com/webview/webview"
)

func main() {
	view := webview.New(false)
	defer view.Destroy()
	view.SetTitle(os.Args[2])
	w, _ := strconv.ParseInt(os.Args[3], 10, 64)
	h, _ := strconv.ParseInt(os.Args[4], 10, 64)
	if w != 0 && h != 0 {
		view.SetSize(int(w), int(h), webview.HintFixed)
	}
	view.Navigate(os.Args[1])
	view.Run()
}
