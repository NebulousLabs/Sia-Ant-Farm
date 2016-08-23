package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"sync"
)

// AntConfig contains fields to pass to a sia-ant job runner.
type AntConfig struct {
	APIAddr      string `json:",omitempty"`
	RPCAddr      string `json:",omitempty"`
	HostAddr     string `json:",omitempty"`
	SiaDirectory string `json:",omitempty"`
	Jobs         []string
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
// `jobs` are passed as flags to sia-ant.  If APIAddr, RPCAddr, or HostAddr are
// defined in `config`, they will be passed to the Ant.  Otherwise, the Ant
// will be passed 3 unused addresses.
func NewAnt(config AntConfig) (*exec.Cmd, error) {
	var args []string
	for _, job := range config.Jobs {
		args = append(args, "-"+job)
	}
	// Create a new temporary directory in the current working directory for this
	// ant's data
	siadir, err := ioutil.TempDir("./antfarm-data", "ant")
	if err != nil {
		return nil, err
	}

	if config.SiaDirectory != "" {
		siadir = config.SiaDirectory
	}

	// Automatically generate 3 free operating system ports for the Ant's api,
	// rpc, and host addresses
	addrs, err := getAddrs(3)
	if err != nil {
		return nil, err
	}
	apiaddr := addrs[0]
	rpcaddr := addrs[1]
	hostaddr := addrs[2]

	// Override the automatically generated addresses with the ones in AntConfig,
	// if they exist.
	if config.APIAddr != "" {
		apiaddr = config.APIAddr
	}
	if config.RPCAddr != "" {
		rpcaddr = config.RPCAddr
	}
	if config.HostAddr != "" {
		hostaddr = config.HostAddr
	}

	fmt.Printf("APIAddr: %v RPCAddr: %v HostAddr: %v\n", apiaddr, rpcaddr, hostaddr)

	args = append(args, "-api-addr", apiaddr, "-rpc-addr", rpcaddr, "-host-addr", hostaddr, "-sia-directory", siadir)
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
		fmt.Fprintf(os.Stderr, "error opening %v: %v\n", *configPath, err)
		os.Exit(1)
	}

	if err = json.NewDecoder(f).Decode(&antConfigs); err != nil {
		fmt.Fprintf(os.Stderr, "error decoding %v: %v\n", *configPath, err)
		os.Exit(1)
	}
	f.Close()

	// Clear out the old antfarm data before starting the new antfarm.
	os.RemoveAll("./antfarm-data")

	// Start each sia-ant process with its assigned jobs from the config file.
	var wg sync.WaitGroup
	var antCommands []*exec.Cmd
	for antindex, config := range antConfigs {
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
