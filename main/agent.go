package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/cloudfoundry/bosh-utils/logger"
	boshsys "github.com/cloudfoundry/bosh-utils/system"

	boshapp "github.com/cloudfoundry/bosh-agent/v2/app"
	"github.com/cloudfoundry/bosh-agent/v2/infrastructure/agentlogger"
	"github.com/cloudfoundry/bosh-agent/v2/platform"
)

const mainLogTag = "main"

func runAgent(opts boshapp.Options, logger logger.Logger) chan error {
	errCh := make(chan error, 1)

	go func() {
		defer logger.HandlePanic("Main")

		logger.Debug(mainLogTag, "Starting agent")

		fs := boshsys.NewOsFileSystem(logger)
		if opts.PlatformName == "dummy" {
			fs = platform.DummyWrapFs(fs)
		}

		app := boshapp.New(logger, fs)
		err := app.Setup(opts)
		if err != nil {
			logger.Error(mainLogTag, "App setup %s", err.Error())
			errCh <- err
			return
		}

		err = app.Run()
		if err != nil {
			logger.Error(mainLogTag, "App run %s", err.Error())
			errCh <- err
			return
		}
	}()
	return errCh
}

func startAgent(logger logger.Logger) error {
	opts, err := boshapp.ParseOptions(os.Args)
	if err != nil {
		logger.Error(mainLogTag, "Parsing options %s", err.Error())
		return err
	}

	if opts.VersionCheck {
		fmt.Println(VersionLabel)
		os.Exit(0)
	}

	sigCh := make(chan os.Signal, 8)
	// `os.Kill` can not be intercepted on UNIX OS's, possibly necessary for Windows?
	signal.Notify(sigCh, syscall.SIGTERM, os.Interrupt, os.Kill) //nolint:staticcheck
	errCh := runAgent(opts, logger)
	for {
		select {
		case sig := <-sigCh:
			return fmt.Errorf("received signal (%s): stopping now", sig)
		case err := <-errCh:
			return err
		}
	}
}

func main() {
	if len(os.Args) > 1 {
		switch cmd := os.Args[1]; cmd {
		case "compile":
			compileTarball(cmd, os.Args[2:])
			return
		}
	}
	asyncLog := logger.NewAsyncWriterLogger(logger.LevelDebug, os.Stderr)
	logger := newSignalableLogger(asyncLog)

	exitCode := 0
	if err := startAgent(logger); err != nil {
		logger.Error(mainLogTag, "Agent exited with error: %s", err)
		exitCode = 1
	}
	if err := logger.FlushTimeout(time.Minute); err != nil {
		logger.Error(mainLogTag, "Setting logger flush timeout failed: %s", err)
	}
	os.Exit(exitCode)
}

func newSignalableLogger(logger logger.Logger) logger.Logger {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGSEGV)
	signalableLogger, _ := agentlogger.NewSignalableLogger(logger, c)
	return signalableLogger
}
