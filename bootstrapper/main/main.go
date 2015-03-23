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

func main() {
	if len(os.Args) != 5 {
		argv0 := os.Args[0]
		fmt.Printf("ERROR - Wrong number of arguments\n\n")
		fmt.Printf("usage: %s <certFile> <keyFile> <caPEM> <allowed distinguished names>\n", argv0)
		fmt.Println()
		fmt.Printf("try this:\n")
		fmt.Printf("%s \\\n", argv0)
		fmt.Printf("   bootstrapper/spec/support/certs/bootstrapper.crt       \\\n")
		fmt.Printf("   bootstrapper/spec/support/certs/bootstrapper.key       \\\n")
		fmt.Printf("   bootstrapper/spec/support/certs/rootCA.pem             \\\n")
		fmt.Printf("   o=bosh.director\n")
		os.Exit(1)
	}

	certFile := os.Args[1]
	keyFile := os.Args[2]
	pemString := os.Args[3]
	allowedName := os.Args[4]

	pem, err := ioutil.ReadFile(pemString)
	if err != nil {
		fmt.Printf("main(): %s\n", err)
		os.Exit(1)
	}

	bootstrapperInstance := &bootstrapper.Bootstrapper{
		CertFile:     certFile,
		KeyFile:      keyFile,
		CACertPem:    (string)(pem),
		AllowedNames: []string{allowedName},

		Logger:           log.New(os.Stderr, "", log.LstdFlags),
		PackageInstaller: package_installer.New(system.NewOsSystem()),
	}

	err = bootstrapperInstance.Listen(4443)
	if err != nil {
		fmt.Printf("main(): %s\n", err)
		os.Exit(1)
	}
	bootstrapperInstance.WaitForServerToExit()
}
