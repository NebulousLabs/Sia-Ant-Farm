package main

import (
	"fmt"
	"time"
)

// gatewayConnectability will print an error to the log if the node has zero
// peers at any time.
func (j *JobRunner) gatewayConnectability() {
	for {
		time.Sleep(time.Second * 5)
		err := j.client.Get("/gateway", nil)
		if err != nil {
			j.errorlog <- fmt.Sprintf("Error in JobPeerConnectability: %v\n", err)
			return
		}
	}
}
