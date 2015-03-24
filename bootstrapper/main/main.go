package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/cloudfoundry/bosh-agent/bootstrapper"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/package_installer"
	"github.com/cloudfoundry/bosh-agent/bootstrapper/system"
)

func usage() {
	argv0 := os.Args[0]
	fmt.Printf("usage: %s <subcommand> <options>\n\n", argv0)
	fmt.Printf("       %s listen <certFile> <keyFile> <caPEM> <allowed distinguished names>\n", argv0)
	fmt.Printf("       %s download <url> <certFile> <keyFile> <caPEM> <allowed distinguished names>\n", argv0)
	// bootstrapper find-metadata
	// bootstrapper do-it-all

	os.Exit(1)
}

func listenUsage() {
	argv0 := os.Args[0]
	fmt.Printf("ERROR - Wrong number of arguments\n\n")
	fmt.Printf("usage: %s listen <certFile> <keyFile> <caPEM> <allowed distinguished names>\n", argv0)
	fmt.Println()
	fmt.Printf("try this:\n")
	fmt.Printf("%s listen \\\n", argv0)
	fmt.Printf("   bootstrapper/spec/support/certs/bootstrapper.crt       \\\n")
	fmt.Printf("   bootstrapper/spec/support/certs/bootstrapper.key       \\\n")
	fmt.Printf("   bootstrapper/spec/support/certs/rootCA.pem             \\\n")
	fmt.Printf("   o=bosh.director\n")
	os.Exit(1)
}

func downloadUsage() {
	argv0 := os.Args[0]
	fmt.Printf("ERROR - Wrong number of arguments\n\n")
	fmt.Printf("usage: %s download <url> <certFile> <keyFile> <caPEM> <allowed distinguished names>\n", argv0)
	fmt.Println()
	fmt.Printf("try this:\n")
	fmt.Printf("%s download https://example.com/something.tgz \\\n", argv0)
	fmt.Printf("   bootstrapper/spec/support/certs/bootstrapper.crt       \\\n")
	fmt.Printf("   bootstrapper/spec/support/certs/bootstrapper.key       \\\n")
	fmt.Printf("   bootstrapper/spec/support/certs/rootCA.pem             \\\n")
	fmt.Printf("   o=bosh.director\n")
	os.Exit(1)
}

func main() {
	if len(os.Args) < 2 {
		usage()
	}

	subcommand := os.Args[1]
	switch subcommand {
	case "listen":
		if len(os.Args) != 6 {
			listenUsage()
		}

		certFile, keyFile, pemFile, allowedName := os.Args[2], os.Args[3], os.Args[4], os.Args[5]

		pem, err := ioutil.ReadFile(pemFile)
		if err != nil {
			fmt.Printf("main(): %s\n", err)
			os.Exit(1)
		}

		k := &bootstrapper.Bootstrapper{
			CertFile:     certFile,
			KeyFile:      keyFile,
			CACertPem:    (string)(pem),
			AllowedNames: []string{allowedName},

			Logger:           log.New(os.Stdout, "", log.LstdFlags),
			PackageInstaller: package_installer.New(system.NewOsSystem()),
		}

		err = k.Listen(4443)
		if err != nil {
			fmt.Printf("main(): %s\n", err)
			os.Exit(1)
		}
		k.WaitForServerToExit()

	case "download":
		if len(os.Args) != 7 {
			downloadUsage()
		}

		url, certFile, keyFile, pemFile, allowedName := os.Args[2], os.Args[3], os.Args[4], os.Args[5], os.Args[6]

		pem, err := ioutil.ReadFile(pemFile)
		if err != nil {
			fmt.Printf("main(): %s\n", err)
			os.Exit(1)
		}

		k := &bootstrapper.Bootstrapper{
			CertFile:     certFile,
			KeyFile:      keyFile,
			CACertPem:    (string)(pem),
			AllowedNames: []string{allowedName},

			Logger:           log.New(os.Stdout, "", log.LstdFlags),
			PackageInstaller: package_installer.New(system.NewOsSystem()),
		}

		err = k.Download(url)
		if err != nil {
			fmt.Printf("main(): %s\n", err)
			os.Exit(1)
		}

	default:
		usage()
	}

	os.Exit(0)
}
