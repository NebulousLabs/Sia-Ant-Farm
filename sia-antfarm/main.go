package main

import (
	"encoding/json"
	"flag"
	"fmt"
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
	var wg sync.WaitGroup
	var ants []*Ant
	for antindex, config := range antfarmConfig.AntConfigs {
		fmt.Printf("Starting ant %v with jobs %v\n", antindex, config.Jobs)
		antcmd, err := NewAnt(config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error starting ant %v: %v\n", antindex, err)
			os.Exit(1)
		}
		defer func() {
			antcmd.Process.Signal(os.Interrupt)
		}()
		wg.Add(1)
		ants = append(ants, antcmd)
		go func() {
			antcmd.Wait()
			wg.Done()
		}()
	}

	// Naively wait for all the ants apis to become available
	time.Sleep(time.Second)

	if antfarmConfig.AutoConnect {
		if err = connectAnts(ants...); err != nil {
			fmt.Fprintf(os.Stderr, "error connecting ant: %v\n", err)
		}
	}

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
