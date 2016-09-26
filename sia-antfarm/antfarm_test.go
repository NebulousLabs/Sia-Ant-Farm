package main

import (
	"testing"
)

// verify that createAntfarm() creates a new antfarm correctly.
func TestNewAntfarm(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	config := AntfarmConfig{
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
}
