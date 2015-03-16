package main

import (
	"fmt"
	"github.com/cloudfoundry/bosh-agent/kickstarter"
)

func main() {
	k := &kickstarter.Kickstarter{}
	err := k.Listen(4443)
	if err != nil {
		fmt.Printf("main(): %s\n", err)
	}
	k.WaitForServerToExit()
}
