package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/NebulousLabs/Sia/api"
)

// TestSpawnAnt verifies that new ant processes are created correctly.
func TestSpawnAnt(t *testing.T) {
	testConfig := AntConfig{
		APIAddr:      "localhost:10000",
		RPCAddr:      "localhost:10001",
		HostAddr:     "localhost:10002",
		SiaDirectory: "/tmp/testdir",
		Jobs: []string{
			"gateway",
		},
	}
	cmd, err := NewAnt(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer cmd.Process.Signal(os.Interrupt)

	if cmd.Args[0] != "sia-ant" {
		t.Fatal("first arg of NewAnt's command should be sia-ant")
	}

	var hasApiAddr, hasRPCAddr, hasHostAddr, hasSiaDirectory, hasGatewayJob bool
	for _, arg := range cmd.Args {
		if arg == testConfig.APIAddr {
			hasApiAddr = true
		}
		if arg == testConfig.RPCAddr {
			hasRPCAddr = true
		}
		if arg == testConfig.HostAddr {
			hasHostAddr = true
		}
		if arg == testConfig.SiaDirectory {
			hasSiaDirectory = true
		}
		if arg == "-"+testConfig.Jobs[0] {
			hasGatewayJob = true
		}
	}
	if !hasSiaDirectory {
		t.Fatal("NewAnt did not pass sia-directory flag to sia-ant")
	}
	if !hasApiAddr {
		t.Fatal("NewAnt did not pass api addr flag to sia-ant")
	}
	if !hasRPCAddr {
		t.Fatal("NewAnt did not pass rpc addr flag to sia-ant")
	}
	if !hasHostAddr {
		t.Fatal("NewAnt did not pass host addr flag to sia-ant")
	}
	if !hasGatewayJob {
		t.Fatal("NewAnt did not pass gateway job flag to sia-ant")
	}
}

func TestConnectAnts(t *testing.T) {
	// connectAnts should throw an error if only one ant is provided
	if err := connectAnts(&Ant{}); err == nil {
		t.Fatal("connectAnts didnt throw an error with only one ant")
	}

	// Spin up three ants and test that connectAnts connects the last two to the first one.
	config := AntConfig{}
	ant0, err := NewAnt(config)
	if err != nil {
		t.Fatal(err)
	}
	defer ant0.Process.Signal(os.Interrupt)

	ant1, err := NewAnt(config)
	if err != nil {
		t.Fatal(err)
	}
	defer ant1.Process.Signal(os.Interrupt)

	ant2, err := NewAnt(config)
	if err != nil {
		t.Fatal(err)
	}
	defer ant2.Process.Signal(os.Interrupt)

	// Allow some time for their APIs to become available
	time.Sleep(time.Second * 5)
	err = connectAnts(ant0, ant1, ant2)
	if err != nil {
		t.Fatal(err)
	}

	c := api.NewClient(ant0.apiaddr, "")
	var gatewayInfo api.GatewayGET
	err = c.Get("/gateway", &gatewayInfo)
	if err != nil {
		t.Fatal(err)
	}
	if len(gatewayInfo.Peers) != 2 {
		t.Fatal("expected ant0 gatewayInfo to have the two peers we connected")
	}

	var hasAnt1, hasAnt2 bool
	for _, peer := range gatewayInfo.Peers {
		if fmt.Sprintf("%s", peer.NetAddress) == ant1.rpcaddr {
			hasAnt1 = true
		}
		if fmt.Sprintf("%s", peer.NetAddress) == ant2.rpcaddr {
			hasAnt2 = true
		}
	}
	if !hasAnt1 {
		t.Fatal("expected ant0 to have ant1 as a peer")
	}
	if !hasAnt2 {
		t.Fatal("expected ant0 to have and2 as a peer")
	}
}
