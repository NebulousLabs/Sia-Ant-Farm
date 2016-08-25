package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"

	"github.com/NebulousLabs/Sia/api"
)

// Ant defines the fields used by a Sia Ant.
type Ant struct {
	apiaddr string
	rpcaddr string
	*exec.Cmd
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

// connectAnts connects two or more ants to the first ant in the slice,
// effectively bootstrapping the antfarm.
func connectAnts(ants ...*Ant) error {
	if len(ants) < 2 {
		return errors.New("you must call connectAnts with at least two ants.")
	}
	targetAnt := ants[0]
	c := api.NewClient(targetAnt.apiaddr, "")
	for _, ant := range ants[1:] {
		err := c.Post(fmt.Sprintf("/gateway/connect/%v", ant.rpcaddr), "", nil)
		if err != nil {
			return err
		}
	}
	return nil
}

// NewAnt spawns a new sia-ant process using os/exec.  The jobs defined by
// `jobs` are passed as flags to sia-ant.  If APIAddr, RPCAddr, or HostAddr are
// defined in `config`, they will be passed to the Ant.  Otherwise, the Ant
// will be passed 3 unused addresses.
func NewAnt(config AntConfig) (*Ant, error) {
	var args []string
	for _, job := range config.Jobs {
		args = append(args, "-"+job)
	}

	// if config.SiaDirectory isn't set, use ioutil.TempDir to create a new
	// temporary directory.
	siadir := config.SiaDirectory
	if siadir == "" {
		os.Mkdir("./antfarm-data", 0700)
		tempdir, err := ioutil.TempDir("./antfarm-data", "ant")
		if err != nil {
			return nil, err
		}
		siadir = tempdir
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

	return &Ant{apiaddr, rpcaddr, cmd}, nil
}
