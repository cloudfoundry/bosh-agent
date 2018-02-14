package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/Microsoft/hcsshim"
)

const NetworkName = "integration-test"

func main() {
	if os.Args[1] == "create" {
		if err := create(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else if os.Args[1] == "destroy" {
		if err := destroy(); err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		fmt.Printf("Usage: %s (create|destroy)\n", os.Args[0])
	}
}

func create() error {
	network := &hcsshim.HNSNetwork{
		Name:    NetworkName,
		Type:    "nat",
		Subnets: []hcsshim.Subnet{{AddressPrefix: "172.30.0.0/24", GatewayAddress: "172.30.0.1"}},
	}

	createdNetwork, err := network.Create()
	if err != nil {
		return fmt.Errorf("failed to create network: %s", err.Error())
	}

	marshaled, err := json.Marshal(createdNetwork)
	if err != nil {
		return fmt.Errorf("failed to JSON marshal information for network %#v: %s", createdNetwork, err.Error())
	}

	fmt.Printf("%s\n", marshaled)

	return nil
}

func destroy() error {
	network, err := hcsshim.GetHNSNetworkByName(NetworkName)
	if err != nil {
		if _, ok := err.(hcsshim.NetworkNotFoundError); ok {
			return fmt.Errorf("could not find network %s", NetworkName)
		}

		return fmt.Errorf("failed to get network: %s", err.Error())
	}

	if _, err := network.Delete(); err != nil {
		return fmt.Errorf("failed to delete network: %s", err.Error())
	}

	return nil
}
