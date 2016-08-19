package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"sync"
)

// AntConfig contains fields to pass to a sia-ant job runner.
type AntConfig struct {
	Jobs []string `json: "jobs"`
}

// NewAnt spawns a new sia-ant process using os/exec.  The jobs defined by
// `jobs` are passed as flags to sia-ant.
func NewAnt(jobs []string) (*exec.Cmd, error) {
	var jobflags []string
	for _, job := range jobs {
		jobflags = append(jobflags, "-"+job)
	}
	cmd := exec.Command("sia-ant", jobflags...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
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
	fmt.Printf("Starting up %v ants...\n", len(antConfigs))
	var wg sync.WaitGroup
	var antCommands []*exec.Cmd
	for _, config := range antConfigs {
		antcmd, err := NewAnt(config.Jobs)
		if err != nil {
			panic(err)
		}
		wg.Add(1)
		antCommands = append(antCommands, antcmd)
		go func() {
			antcmd.Wait()
			wg.Done()
		}()
	}

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)

	go func() {
		<-sigchan
		for _, cmd := range antCommands {
			cmd.Process.Signal(os.Interrupt)
		}
	}()

	wg.Wait()
}
