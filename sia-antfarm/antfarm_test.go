package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/NebulousLabs/Sia/api"
)

// verify that createAntfarm() creates a new antfarm correctly.
func TestNewAntfarm(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	config := AntfarmConfig{
		ListenAddress: "localhost:31337",
		AntConfigs: []AntConfig{
			{
				RPCAddr: "localhost:3337",
				Jobs: []string{
					"gateway",
				},
			},
		},
	}

	antfarm, err := createAntfarm(config)
	if err != nil {
		t.Fatal(err)
	}
	defer antfarm.Close()

	go antfarm.ServeAPI()

	res, err := http.DefaultClient.Get("http://localhost:31337/ants")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()

	var ag antsGET
	err = json.NewDecoder(res.Body).Decode(&ag)
	if err != nil {
		t.Fatal(err)
	}
	if len(ag.Ants) != len(config.AntConfigs) {
		t.Fatal("expected /ants to return the correct number of ants")
	}
	if ag.Ants[0].RPCAddr != config.AntConfigs[0].RPCAddr {
		t.Fatal("expected /ants to return the correct rpc address")
	}
}

// verify that connectExternalAntfarm connects antfarms to eachother correctly
func TestConnectExternalAntfarm(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	config1 := AntfarmConfig{
		ListenAddress: "127.0.0.1:31337",
		AntConfigs: []AntConfig{
			{
				RPCAddr: "127.0.0.1:3337",
				Jobs: []string{
					"gateway",
				},
			},
		},
	}

	config2 := AntfarmConfig{
		ListenAddress: "127.0.0.1:31338",
		AntConfigs: []AntConfig{
			{
				RPCAddr: "127.0.0.1:3338",
				Jobs: []string{
					"gateway",
				},
			},
		},
	}

	farm1, err := createAntfarm(config1)
	if err != nil {
		t.Fatal(err)
	}
	defer farm1.Close()
	go farm1.ServeAPI()

	farm2, err := createAntfarm(config2)
	if err != nil {
		t.Fatal(err)
	}
	defer farm2.Close()
	go farm2.ServeAPI()

	err = farm1.connectExternalAntfarm(config2.ListenAddress)
	if err != nil {
		t.Fatal(err)
	}

	// give a bit of time for the connection to succeed
	time.Sleep(time.Second * 3)

	// verify that farm2 has farm1 as its peer
	c := api.NewClient(farm1.ants[0].APIAddr, "")
	var gatewayInfo api.GatewayGET
	err = c.Get("/gateway", &gatewayInfo)
	if err != nil {
		t.Fatal(err)
	}

	for _, ant := range farm2.ants {
		hasAddr := false
		for _, peer := range gatewayInfo.Peers {
			if fmt.Sprintf("%s", peer.NetAddress) == ant.RPCAddr {
				hasAddr = true
			}
		}
		if !hasAddr {
			t.Fatalf("farm1 is missing %v", ant.RPCAddr)
		}
	}
}
