package main

import (
	"log"
	"time"

	"github.com/NebulousLabs/Sia/api"
)

// gatewayConnectability will print an error to the log if the node has zero
// peers at any time.
func (j *JobRunner) gatewayConnectability() {
	for {
		time.Sleep(time.Second * 5)
		var gatewayInfo api.GatewayGET
		err := j.client.Get("/gateway", &gatewayInfo)
		if err != nil {
			log.Printf("[%v gatewayConnectability ERROR]: %v\n", j.siaDirectory, err)
			return
		}
		if len(gatewayInfo.Peers) == 0 {
			log.Printf("[%v gatewayConnectability WARN]: ant has zero peers", j.siaDirectory)
		}
	}
}
