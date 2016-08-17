package main

import (
	"fmt"
	"time"

	"github.com/NebulousLabs/Sia/api"
)

type JobRunner struct {
	client   *api.Client
	errorlog chan interface{}
}

func NewJobRunner(apiaddr string, authpassword string) *JobRunner {
	return &JobRunner{
		errorlog: make(chan interface{}),
		client:   api.NewClient(apiaddr, authpassword),
	}
}

// gatewayConnectability will print an error to the log if the node has zero
// peers at any time.
func (j *JobRunner) gatewayConnectability() {
	for {
		time.Sleep(time.Second * 5)
		var info api.GatewayGET
		err := j.client.Get("/gateway", &info)
		if err != nil {
			j.errorlog <- fmt.Sprintf("Error in JobPeerConnectability: %v\n", err)
			return
		}
		if len(info.Peers) == 0 {
			j.errorlog <- "JobPeerConnectability: node has zero peers..."
		}
	}
}
