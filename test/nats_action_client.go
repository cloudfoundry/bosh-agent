package main

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"os"
	"time"

	"github.com/nats-io/go-nats"
)

func main() {
	certFile := "./nats_client_certificate.pem"
	keyFile := "./nats_client_private_key"
	cert, err := tls.LoadX509KeyPair(certFile, keyFile)
	if err != nil {
		fmt.Printf("Error: error parsing X509 certificate/key pair: %v\n", err)
		return
	}

	if len(os.Args) < 6 {
		fmt.Printf("Error: Usage requires arguments <nats IP> <agent channel ID> <reply to ID> <command name> <command arguments string>\n")
		return
	}

	natsIP := os.Args[1]
	agentId := os.Args[2]
	replyToId := os.Args[3]
	requestCommandName := os.Args[4]
	requestCommandArgumentsString := os.Args[5]
	fmt.Printf("Requesting access to agent '%v'\nreply to is '%v'\nwith request command '%v'\nwith request command arguments '%v'\n", agentId, replyToId, requestCommandName, requestCommandArgumentsString)

	bs, err := ioutil.ReadFile("./nats_server_ca.pem")
	if err != nil {
		fmt.Printf("Error: error parsing X509 certificate/key pair: %v\n", err)
		return
	}

	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(bs)

	config := &tls.Config{
		ServerName:   natsIP,
		Certificates: []tls.Certificate{cert},
		RootCAs:      pool,
		MinVersion:   tls.VersionTLS12,
	}

	nc, err := nats.Connect("nats://"+natsIP+":4222", nats.Secure(config))
	if err != nil {
		fmt.Printf("Error: Got an error on Connect with Secure Options: %+v\n", err)
		return
	}
	defer nc.Close()

	payload := []byte(`{"protocol":3,"method":"` + requestCommandName + `","arguments":` + requestCommandArgumentsString + `,"reply_to":"` + replyToId + `"}`)
	fmt.Printf("\nusing payload %s\n", payload)
	// payload := []byte(`{"protocol":3,"method":"mount_disk","arguments":["fake_disk_id_for_acceptance", "/device-path"],"reply_to":"director.da40148a-ea98-4b69-8395-fa47b1cc8387.c2674c03-f8e8-454a-a3d8-8d8994b4596d.92c4791c-9534-43a3-8dfb-2c9b72e280a8"}`)
	// payload := []byte(`{"protocol":3,"method":"mount_disk","arguments":["fake_disk_id_for_acceptance", {"path": "/device-path"}],"reply_to":"director.da40148a-ea98-4b69-8395-fa47b1cc8387.c2674c03-f8e8-454a-a3d8-8d8994b4596d.92c4791c-9534-43a3-8dfb-2c9b72e280a8"}`)

	msg, err := nc.Request(agentId, payload, 10000*time.Millisecond)
	if err != nil {
		fmt.Printf("Error: request %s\n", err)
		return
	}

	fmt.Printf("Received [%v] : '%s'\n", msg.Subject, string(msg.Data))
}
