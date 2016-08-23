package main

import (
	"os"
	"testing"
)

// TestSpawnAnt verifies that new ant processes are created correctly.
func TestSpawnAnt(t *testing.T) {
	testConfig := AntConfig{
		APIAddr:  "localhost:10000",
		RPCAddr:  "localhost:10001",
		HostAddr: "localhost:10002",
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

	var hasApiAddr, hasRPCAddr, hasHostAddr, hasGatewayJob bool
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
		if arg == "-"+testConfig.Jobs[0] {
			hasGatewayJob = true
		}
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
