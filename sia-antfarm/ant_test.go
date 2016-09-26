package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"reflect"
	"testing"
	"time"

	"github.com/NebulousLabs/Sia/api"
)

// TestStartAnts verifies that startAnts successfully starts ants given some
// configs.
func TestStartAnts(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	configs := []AntConfig{
		{},
		{},
		{},
	}

	ants, err := startAnts(configs...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, ant := range ants {
			ant.Process.Signal(os.Interrupt)
			ant.Wait()
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

	configs := []AntConfig{
		{},
		{},
		{},
		{},
		{},
	}

	ants, err := startAnts(configs...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, ant := range ants {
			ant.Process.Signal(os.Interrupt)
			ant.Wait()
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
	configs := []AntConfig{
		{},
		{},
		{},
	}
	ants, err := startAnts(configs...)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		for _, ant := range ants {
			ant.Process.Signal(os.Interrupt)
			ant.Wait()
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
	otherAnt, err := NewAnt(AntConfig{APIAddr: "", RPCAddr: "", HostAddr: "", SiaDirectory: "", Jobs: []string{"mining"}})
	if err != nil {
		t.Fatal(err)
	}
	ants = append(ants, otherAnt)

	// Wait for the other ant to mine a few blocks
	time.Sleep(time.Second * 10)

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
