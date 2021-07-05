package main

import (
	"os"
	_ "time/tzdata"

	"github.com/wzshiming/jumpway/app/tray"
	"github.com/wzshiming/jumpway/daemon"
	"github.com/wzshiming/jumpway/log"
)

func main() {
	log.Info("Args", "list", os.Args)
	if len(os.Args) == 2 {
		daemon.Run(os.Args[1])
		return
	}

	a := tray.NewApp()
	a.Run()
}
