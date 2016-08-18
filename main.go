package main

import (
	"encoding/json"
	"flag"
	"os"
	"os/exec"
	"os/signal"
)

// AntConfig contains fields to pass to a sia-ant job runner.
type AntConfig struct {
	Jobs []string `json: "jobs"`
}

// NewAnt spawns a new sia-ant process using os/exec.  The jobs defined by
// `jobs` are passed as flags to sia-ant.
func NewAnt(jobs []string) (*exec.Cmd, error) {
	cmd := exec.Command("sia-ant", jobs...)
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	return cmd, nil
}

func main() {
	configPath := flag.String("config", "config.json", "path to the sia-antfarm configuration file")

	flag.Parse()

	// Read and decode the sia-antfarm configuration file.
	var antConfigs []AntConfig
	f, err := os.Open(*configPath)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	if err = json.NewDecoder(f).Decode(&antConfigs); err != nil {
		panic(err)
	}

	// Start each sia-ant process with its assigned jobs from the config file.
	var antProcesses []*os.Process
	for _, config := range antConfigs {
		antcmd, err := NewAnt(config.Jobs)
		if err != nil {
			panic(err)
		}
		antProcesses = append(antProcesses, andcmd.Process)
	}

	// Signal each sia-ant process to exit when ctrl-c is input to sia-antfarm.
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.interrupt)

	go func() {
		<-sigchan
		for _, process := range antProcesses {
			process.Kill()
		}
	}()

	// Wait on the main thread for every sia-ant process to complete.
	for _, process := range antProcesses {
		if err = process.Wait(); err != nil && err.Error != "signal: killed" {
			panic(err)
		}
	}
}
