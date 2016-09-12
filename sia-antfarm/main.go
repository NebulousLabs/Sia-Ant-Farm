package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"time"
)

// AntConfig contains fields to pass to a sia-ant job runner.
type AntConfig struct {
	APIAddr      string `json:",omitempty"`
	RPCAddr      string `json:",omitempty"`
	HostAddr     string `json:",omitempty"`
	SiaDirectory string `json:",omitempty"`
	Jobs         []string
}

// AntfarmConfig contains the fields to parse and use to create a sia-antfarm.
type AntfarmConfig struct {
	AntConfigs  []AntConfig
	AutoConnect bool
}

func main() {
	configPath := flag.String("config", "config.json", "path to the sia-antfarm configuration file")

	flag.Parse()

	// Read and decode the sia-antfarm configuration file.
	var antfarmConfig AntfarmConfig
	f, err := os.Open(*configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error opening %v: %v\n", *configPath, err)
		os.Exit(1)
	}

	if err = json.NewDecoder(f).Decode(&antfarmConfig); err != nil {
		fmt.Fprintf(os.Stderr, "error decoding %v: %v\n", *configPath, err)
		os.Exit(1)
	}
	f.Close()

	// Clear out the old antfarm data before starting the new antfarm.
	os.RemoveAll("./antfarm-data")

	// Start each sia-ant process with its assigned jobs from the config file.
	ants, err := startAnts(antfarmConfig.AntConfigs...)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error starting ants: %v\n", err)
		os.Exit(1)
	}

	// Wait for every ant process to exit before exiting antfarm
	var wg sync.WaitGroup
	wg.Add(len(ants))
	go func() {
		for _, ant := range ants {
			ant.Wait()
			wg.Done()
		}
	}()

	if antfarmConfig.AutoConnect {
		if err = connectAnts(ants...); err != nil {
			fmt.Fprintf(os.Stderr, "error connecting ant: %v\n", err)
		}
	}

	// Spawn a thread that checks that all ants in the antfarm are on the same
	// blockchain every 10 seconds.
	go func() {
		for {
			time.Sleep(time.Second * 10)
			synced, err := antsAreSynced(ants...)
			if err != nil {
				log.Println("error checking sync status of antfarm: ", err)
				continue
			}
			if !synced {
				log.Println("WARN: ant desynchronization detected!")
			}
		}
	}()

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)

	go func() {
		<-sigchan
		fmt.Println("Caught quit signal, stopping all ants...")
		for _, cmd := range ants {
			cmd.Process.Signal(os.Interrupt)
		}
	}()

	fmt.Printf("Finished.  Running sia-antfarm with %v ants.\n", len(antfarmConfig.AntConfigs))

	wg.Wait()
}
