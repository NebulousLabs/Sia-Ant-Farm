package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"testing"

	"github.com/NebulousLabs/Sia/api"
)

// TestSpawnAnt verifies that new ant processes are created correctly.
func TestSpawnAnt(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	datadir, err := ioutil.TempDir("", "sia-testing")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(datadir)

	testConfig := AntConfig{
		APIAddr:      "localhost:10000",
		RPCAddr:      "localhost:10001",
		HostAddr:     "localhost:10002",
		SiaDirectory: datadir,
		Jobs: []string{
			"gateway",
		},
	}

	cmd, err := NewAnt(testConfig)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cmd.Process.Signal(os.Interrupt)
		cmd.Wait()
	}()

	// Verify the API is reachable after NewAnt returns
	c := api.NewClient(cmd.apiaddr, "")
	if err = c.Get("/consensus", nil); err != nil {
		t.Fatal(err)
	}

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
	if testing.Short() {
		t.SkipNow()
	}

	// connectAnts should throw an error if only one ant is provided
	if err := connectAnts(&Ant{}); err == nil {
		t.Fatal("connectAnts didnt throw an error with only one ant")
	}

	n_ants := 5
	config := AntConfig{}
	var ants []*Ant

	for i := 0; i < n_ants; i++ {
		ant, err := NewAnt(config)
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			ant.Process.Signal(os.Interrupt)
			ant.Wait()
		}()
		ants = append(ants, ant)
	}

	err := connectAnts(ants...)
	if err != nil {
		t.Fatal(err)
	}

	c := api.NewClient(ants[0].apiaddr, "")
	var gatewayInfo api.GatewayGET
	err = c.Get("/gateway", &gatewayInfo)
	if err != nil {
		t.Fatal(err)
	}

	for _, ant := range ants[1:] {
		hasAddr := false
		for _, peer := range gatewayInfo.Peers {
			if fmt.Sprintf("%s", peer.NetAddress) == ant.rpcaddr {
				hasAddr = true
			}
		}
		if !hasAddr {
			t.Fatalf("the central ant is missing %v", ant.rpcaddr)
		}
	}
}
