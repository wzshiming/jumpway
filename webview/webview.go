package webview

import (
	"os"
	"strconv"

	"github.com/getlantern/byteexec"
	"github.com/skratchdot/open-golang/open"
	"github.com/wzshiming/logger"
)

func View(url string, title string, w, h int64) error {
	exec, err := byteexec.New(view, os.Args[0])
	if err != nil {
		logger.Log.Error(err, "byte exec")
		return open.Start(url)
	}
	return exec.Command(url, title, strconv.FormatInt(w, 10), strconv.FormatInt(h, 10)).Run()
}
