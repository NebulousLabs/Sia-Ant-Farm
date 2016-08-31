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

	for {
		select {
		case <-j.tg.StopChan():
			return
		case <-time.After(time.Second * 5):
		}

		var gatewayInfo api.GatewayGET
		err := j.client.Get("/gateway", &gatewayInfo)
		if err != nil {
			log.Printf("[%v gatewayConnectability ERROR]: %v\n", j.siaDirectory, err)
		}
		if len(gatewayInfo.Peers) == 0 {
			log.Printf("[%v gatewayConnectability WARN]: ant has zero peers", j.siaDirectory)
		}
	}
}
