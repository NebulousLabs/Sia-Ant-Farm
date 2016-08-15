package main

import (
	"fmt"
	"time"

	"github.com/NebulousLabs/Sia/api"
)

// JobPeerConnectability will print an error to the log if the node has zero
// peers at any time.
func JobPeerConnectability(log chan interface{}) {
	for {
		time.Sleep(time.Second * 5)
		var info api.GatewayGET
		err := getAPI("/gateway", &info)
		if err != nil {
			log <- fmt.Sprintf("Error in JobPeerConnectability: %v\n", err)
			return
		}
		if len(info.Peers) == 0 {
			log <- "JobPeerConnectability: node has zero peers..."
		}
	}
}
