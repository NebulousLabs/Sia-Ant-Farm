package main

import (
	"log"
	"time"
)

// gatewayConnectability will print an error to the log if the node has zero
// peers at any time.
func (j *JobRunner) gatewayConnectability() {
	for {
		time.Sleep(time.Second * 5)
		err := j.client.Get("/gateway", nil)
		if err != nil {
			log.Printf("[gatewayConnectability ERROR]: %v\n", err)
			return
		}
	}
}
