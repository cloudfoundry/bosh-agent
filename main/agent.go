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
	boshfirewall "github.com/cloudfoundry/bosh-agent/v2/platform/firewall"
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
		case "firewall-allow":
			handleFirewallAllow(os.Args[2:])
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

// handleFirewallAllow handles the "bosh-agent firewall-allow <service>" CLI command.
// This is called by processes (like monit) that need firewall access to local services.
func handleFirewallAllow(args []string) {
	if len(args) < 1 {
		fmt.Fprintf(os.Stderr, "Usage: bosh-agent firewall-allow <service>\n")
		fmt.Fprintf(os.Stderr, "Allowed services: %v\n", boshfirewall.AllowedServices)
		os.Exit(1)
	}

	service := boshfirewall.Service(args[0])

	// Create minimal logger for CLI command
	log := logger.NewLogger(logger.LevelError)

	// Create firewall manager
	firewallMgr, err := boshfirewall.NewNftablesFirewall(log)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error creating firewall manager: %s\n", err)
		os.Exit(1)
	}

	// Get parent PID (the process that called us)
	callerPID := os.Getppid()

	if err := firewallMgr.AllowService(service, callerPID); err != nil {
		fmt.Fprintf(os.Stderr, "Error allowing service: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Firewall exception added for service: %s (caller PID: %d)\n", service, callerPID)
}
