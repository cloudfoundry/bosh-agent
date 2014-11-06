package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"runtime"
	"strconv"

	boshapp "github.com/cloudfoundry/bosh-agent/app"
	boshlog "github.com/cloudfoundry/bosh-agent/logger"
)

const mainLogTag = "main"

func main() {
	logger := boshlog.NewLogger(boshlog.LevelDebug)
	defer logger.HandlePanic("Main")

	runtime.GOMAXPROCS(2)
	logger.Debug(mainLogTag, "GOMAXPROCS set to 2")

	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()

	http.HandleFunc("/gomaxprocs", func(w http.ResponseWriter, r *http.Request) {
		err := r.ParseForm()
		if err != nil {
			w.Header().Set("Status-Code", "500")
			io.WriteString(w, fmt.Sprintf("WTF: %s", err.Error()))
		}

		newGoMaxProcs, err := strconv.Atoi(r.Form.Get("num"))
		if err != nil {
			w.Header().Set("Status-Code", "500")
			io.WriteString(w, fmt.Sprintf("WTF: %s", err.Error()))
			return
		}
		runtime.GOMAXPROCS(newGoMaxProcs)
		effectiveGoMaxProcs := runtime.GOMAXPROCS(-1)
		w.Header().Set("Status-Code", "200")
		io.WriteString(w, fmt.Sprintf("%d", effectiveGoMaxProcs))
	})

	go func() {
		err := http.ListenAndServe(":8080", nil)
		logger.Error(mainLogTag, "starting http server %s", err.Error())
		os.Exit(1)
	}()

	logger.Debug(mainLogTag, "Starting agent")

	app := boshapp.New(logger)

	err := app.Setup(os.Args)
	if err != nil {
		logger.Error(mainLogTag, "App setup %s", err.Error())
		os.Exit(1)
	}

	err = app.Run()
	if err != nil {
		logger.Error(mainLogTag, "App run %s", err.Error())
		os.Exit(1)
	}
}
