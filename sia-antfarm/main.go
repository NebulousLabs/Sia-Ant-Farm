package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
)

// AntConfig contains fields to pass to a sia-ant job runner.
type AntConfig struct {
	Jobs []string `json: "jobs"`
}

// getAddrs returns n free listening addresses on localhost by leveraging the
// behaviour of net.Listen("localhost:0").
func getAddrs(n int) ([]string, error) {
	var addrs []string

	for i := 0; i < n; i++ {
		l, err := net.Listen("tcp", "localhost:0")
		if err != nil {
			return nil, err
		}
		defer l.Close()
		addrs = append(addrs, l.Addr().String())
	}
	return addrs, nil
}

// NewAnt spawns a new sia-ant process using os/exec.  The jobs defined by
// `jobs` are passed as flags to sia-ant.
func NewAnt(jobs []string) (*exec.Cmd, error) {
	var args []string
	for _, job := range jobs {
		args = append(args, "-"+job)
	}
	addrs, err := getAddrs(3)
	if err != nil {
		return nil, err
	}
	args = append(args, "-api-addr", addrs[0], "-rpc-addr", addrs[1], "-host-addr", addrs[2])
	cmd := exec.Command("sia-ant", args...)
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
	var wg sync.WaitGroup
	var antCommands []*exec.Cmd
	for antindex, config := range antConfigs {
		fmt.Printf("Starting ant %v with jobs %v\n", antindex, config.Jobs)
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
		fmt.Println("Caught quit signal, stopping all ants...")
		for _, cmd := range antCommands {
			cmd.Process.Signal(os.Interrupt)
		}
	}()

	fmt.Printf("Finished.  Running sia-antfarm with %v ants.\n", len(antConfigs))

	wg.Wait()
}
