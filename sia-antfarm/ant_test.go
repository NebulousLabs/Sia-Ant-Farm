package main

import (
	"fmt"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/NebulousLabs/Sia-Ant-Farm/ant"
	"github.com/NebulousLabs/Sia/node/api"
)

// TestStartAnts verifies that startAnts successfully starts ants given some
// configs.
func TestStartAnts(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	configs := []ant.AntConfig{
		{},
		{},
		{},
	}

	os.MkdirAll("./antfarm-data", 0700)
	defer os.RemoveAll("./antfarm-data")

	ants, err := startAnts(configs...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, ant := range ants {
			ant.Close()
		}
	}()

	// verify each ant has a reachable api
	for _, ant := range ants {
		c := api.NewClient(ant.APIAddr, "")
		if err := c.Get("/consensus", nil); err != nil {
			t.Fatal(err)
		}
	}
}

func TestConnectAnts(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	// connectAnts should throw an error if only one ant is provided
	if err := connectAnts(&ant.Ant{}); err == nil {
		t.Fatal("connectAnts didnt throw an error with only one ant")
	}

	configs := []ant.AntConfig{
		{},
		{},
		{},
		{},
		{},
	}

	os.MkdirAll("./antfarm-data", 0700)
	defer os.RemoveAll("./antfarm-data")

	ants, err := startAnts(configs...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, ant := range ants {
			ant.Close()
		}
	}()

	err = connectAnts(ants...)
	if err != nil {
		t.Fatal(err)
	}

	c := api.NewClient(ants[0].APIAddr, "")
	var gatewayInfo api.GatewayGET
	err = c.Get("/gateway", &gatewayInfo)
	if err != nil {
		t.Fatal(err)
	}

	for _, ant := range ants[1:] {
		hasAddr := false
		for _, peer := range gatewayInfo.Peers {
			if fmt.Sprintf("%s", peer.NetAddress) == "127.0.0.1"+ant.RPCAddr {
				hasAddr = true
			}
		}
		if !hasAddr {
			t.Fatalf("the central ant is missing %v", "127.0.0.1"+ant.RPCAddr)
		}
	}
}

func TestAntConsensusGroups(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	// spin up our testing ants
	configs := []ant.AntConfig{
		{},
		{},
		{},
	}

	os.MkdirAll("./antfarm-data", 0700)
	defer os.RemoveAll("./antfarm-data")

	ants, err := startAnts(configs...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, ant := range ants {
			ant.Close()
		}
	}()

	groups, err := antConsensusGroups(ants...)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 1 {
		t.Fatal("expected 1 consensus group initially")
	}
	if len(groups[0]) != len(ants) {
		t.Fatal("expected the consensus group to have all the ants")
	}

	// Start an ant that is desynced from the rest of the network
	cfg, err := parseConfig(ant.AntConfig{Jobs: []string{"miner"}})
	if err != nil {
		t.Fatal(err)
	}
	otherAnt, err := ant.New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ants = append(ants, otherAnt)

	// Wait for the other ant to mine a few blocks
	time.Sleep(time.Second * 30)

	groups, err = antConsensusGroups(ants...)
	if err != nil {
		t.Fatal(err)
	}
	if len(groups) != 2 {
		t.Fatal("expected 2 consensus groups")
	}
	if len(groups[0]) != len(ants)-1 {
		t.Fatal("expected the first consensus group to have 3 ants")
	}
	if len(groups[1]) != 1 {
		t.Fatal("expected the second consensus group to have 1 ant")
	}
	if !reflect.DeepEqual(groups[1][0], otherAnt) {
		t.Fatal("expected the miner ant to be in the second consensus group")
	}
}
