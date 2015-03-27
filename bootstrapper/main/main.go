package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"

	"github.com/cloudfoundry/bosh-agent/bootstrapper/auth"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/downloader"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/installer"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/listener"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/system"
	"github.com/cloudfoundry/bosh-agent/logger"
)

var (
	flags       = flag.NewFlagSet("flags", flag.ExitOnError)
	certFile    = flags.String("certFile", "", "path to certificate")
	keyFile     = flags.String("keyFile", "", "path to certificate key")
	caPemFile   = flags.String("caPemFile", "", "path to CA PEM")
	allowedName = flags.String("allowedName", "", "distiguished name to allow")
)

func usage() {
	argv0 := os.Args[0]
	fmt.Printf("bootstrapper usage:\n")
	fmt.Printf("%s listen port <options>\n", argv0)
	fmt.Printf("%s download url <options>\n", argv0)
	fmt.Printf("\noptions:\n")
	flags.PrintDefaults()
	os.Exit(1)
}

func main() {
	logger := newLogger()
	parseFlags(logger)
	installer := installer.New(system.NewOsSystem())
	config := newSSLConfig(logger)

	switch os.Args[1] {
	case "listen":
		portString := os.Args[2]
		port, err := strconv.Atoi(portString)
		if err != nil {
			log.Println("failed to parse port '", portString, "' :", err)
			os.Exit(1)
		}

		l := listener.NewListener(config, installer)
		err = l.ListenAndServe(logger, port)
		if err != nil {
			os.Exit(1)
		}
		l.WaitForServerToExit()

	case "download":
		url := os.Args[2]

		d := downloader.NewDownloader(config, installer)
		err := d.Download(logger, url)
		if err != nil {
			os.Exit(1)
		}

	default:
		usage()
	}

	os.Exit(0)
}

func parseFlags(logger logger.Logger) {
	err := flags.Parse(os.Args[3:])
	if err != nil {
		usage()
	}
	flags.VisitAll(func(f *flag.Flag) {
		if f.Value.String() == "" {
			logger.Error("flags", "%s is a required flag", f.Name)
			usage()
		}
	})
}

func newLogger() logger.Logger {
	log.SetOutput(os.Stdout)
	sysLog := log.New(os.Stdout, "", log.LstdFlags)
	return logger.New(logger.LevelDebug, sysLog, sysLog)
}

func newSSLConfig(logger logger.Logger) auth.SSLConfig {
	pem, err := ioutil.ReadFile(*caPemFile)
	if err != nil {
		logger.Error("CaPEMFile", "failed to read pemFile: ", err)
		os.Exit(1)
	}

	config, err := auth.NewSSLConfig(*certFile, *keyFile, string(pem), []string{*allowedName})
	if err != nil {
		logger.Error("Config", "Unable to create SSL config", err)
		os.Exit(1)
	}
	return config
}
