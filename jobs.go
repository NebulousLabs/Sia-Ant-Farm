package main

import (
	"fmt"
	"time"

	"github.com/NebulousLabs/Sia/api"
)

type JobRunner struct {
	errorlog chan interface{}
}

func NewJobRunner() *JobRunner {
	return &JobRunner{
		errorlog: make(chan interface{}),
	}
}

// gatewayConnectability will print an error to the log if the node has zero
// peers at any time.
func (j *JobRunner) gatewayConnectability() {
	for {
		time.Sleep(time.Second * 5)
		var info api.GatewayGET
		err := getAPI("/gateway", &info)
		if err != nil {
			j.errorlog <- fmt.Sprintf("Error in JobPeerConnectability: %v\n", err)
			return
		}
		if len(info.Peers) == 0 {
			j.errorlog <- "JobPeerConnectability: node has zero peers..."
		}
	}
}
