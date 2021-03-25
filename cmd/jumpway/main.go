package main

import (
	"os"

	"github.com/wzshiming/jumpway/daemon"

	_ "time/tzdata"

	"github.com/wzshiming/jumpway/app"
	"github.com/wzshiming/logger"
)

func main() {
	logger.Log.Info("Args", "list", os.Args)
	if len(os.Args) == 2 {
		daemon.Run(os.Args[1])
		return
	}

	a := app.NewApp()
	a.Run()
}
