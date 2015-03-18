package main

import (
	"fmt"
	"github.com/cloudfoundry/bosh-agent/bootstrapper"
	"io/ioutil"
	"os"
)

func main() {
	if len(os.Args) != 4 {
		argv0 := "bootstrapper"
		fmt.Printf("usage: %s <certFile> <keyFile> <caPEM>\n", argv0)
		fmt.Println()
		fmt.Printf("try this: %s bootstrapper/spec/support/certs/bootstrapper.crt bootstrapper/spec/support/certs/bootstrapper.key bootstrapper/spec/support/certs/rootCA.pem\n", argv0)
		os.Exit(1)
	}

	pem, err := ioutil.ReadFile(os.Args[3])
	if err != nil {
		fmt.Printf("main(): %s\n", err)
		os.Exit(1)
	}

	k := &bootstrapper.Bootstrapper{
		CertFile:  os.Args[1],
		KeyFile:   os.Args[2],
		CACertPem: (string)(pem),
	}

	err = k.Listen(4443)
	if err != nil {
		fmt.Printf("main(): %s\n", err)
		os.Exit(1)
	}
	k.WaitForServerToExit()
}
