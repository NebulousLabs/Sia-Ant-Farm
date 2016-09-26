package main

import (
	"encoding/json"
	"net/http"
	"testing"
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
