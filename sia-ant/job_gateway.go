package main

import (
	"log"
	"time"

	"github.com/NebulousLabs/Sia/api"
)

// gatewayConnectability will print an error to the log if the node has zero
// peers at any time.
func (j *JobRunner) gatewayConnectability() {
	done := make(chan struct{})
	defer close(done)
	j.tg.OnStop(func() {
		<-done
	})

	// Initially wait a while to give the other ants some time to spin up.
	select {
	case <-j.tg.StopChan():
		return
	case <-time.After(time.Minute):
	}

	for {
		// Wait 30 seconds between iterations.
		select {
		case <-j.tg.StopChan():
			return
		case <-time.After(time.Second * 30):
		}

		// Count the number of peers that the gateway has. An error is reported
		// for less than two peers because the gateway is likely connected to
		// itself.
		var gatewayInfo api.GatewayGET
		err := j.client.Get("/gateway", &gatewayInfo)
		if err != nil {
			log.Printf("[ERROR] [gateway] [%v] error when calling /gateway: %v\n", j.siaDirectory, err)
		}
		if len(gatewayInfo.Peers) < 2 {
			log.Printf("[ERROR] [gateway] [%v] ant has less than two peers: %v\n", j.siaDirectory, gatewayInfo.Peers)
		}
	}
}
