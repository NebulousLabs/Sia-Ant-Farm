package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"time"
)

type Siad struct {
	hostAddr string
	rpcAddr  string
	apiAddr  string

	cmd *exec.Cmd
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

// NewSiad spawns a new siad process using os/exec.  siadPath is the path to
// Siad, passed directly to exec.Command.  An error is returned if starting
// siad fails, otherwise a pointer to siad's os.Cmd object is returned.  The
// data directory `datadir` is passed as siad's `--sia-directory`.  3 open
// ports will be assigned as Sia's api, rpc, and host ports.
func NewSiad(siadPath string, datadir string) (*Siad, error) {
	// get 3 available bind addresses
	addrs, err := getAddrs(3)
	if err != nil {
		return nil, err
	}
	siad := &Siad{
		apiAddr:  addrs[0],
		rpcAddr:  addrs[1],
		hostAddr: addrs[2],
	}
	siad.cmd = exec.Command(siadPath, "--sia-directory", datadir, "--api-addr", siad.apiAddr, "--rpc-addr", siad.rpcAddr, "--host-addr", siad.hostAddr)
	siad.cmd.Stdout = os.Stdout
	siad.cmd.Stderr = os.Stderr
	if err := siad.cmd.Start(); err != nil {
		return nil, err
	}
	return siad, nil
}

func main() {
	siadPath := flag.String("siad", "siad", "path to siad executable")
	runGateway := flag.Bool("gateway", false, "enable gateway test jobs")
	runMining := flag.Bool("mining", false, "enable mining test jobs")
	flag.Parse()

	// Create a new temporary directory for ephemeral data storage for this ant.
	datadir, err := ioutil.TempDir("", "sia-antfarm")
	if err != nil {
		panic(err)
	}
	defer func() {
		err := os.RemoveAll(datadir)
		if err != nil {
			panic(err)
		}
	}()

	// Construct a new siad instance
	siad, err := NewSiad(*siadPath, datadir)
	if err != nil {
		panic(err)
		return
	}

	// Naively wait for the daemon to start.
	time.Sleep(time.Second)

	// Construct the job runner
	j, err := NewJobRunner(siad.apiAddr, "")
	if err != nil {
		panic(err)
	}

	// Construct the signal channel and notify on it in the case of SIGINT
	// (ctrl-c) or SIGKILL
	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt, os.Kill)

	// Concurrently print errors or kill siad and quit on ctrl-c
	go func() {
		for {
			select {
			case <-sigchan:
				fmt.Println("Caught quit signal, quitting...")
				siad.cmd.Process.Kill()
				return
			case err := <-j.errorlog:
				fmt.Printf("%v: %v\n", time.Now(), err)
			}
		}
	}()

	fmt.Println("> Starting jobs...")

	// Start up siad jobs
	if *runGateway {
		fmt.Println(">> running gateway connectability job...")
		go j.gatewayConnectability()
	}
	if *runMining {
		fmt.Println(">> running mining job...")
		go j.blockMining()
	}

	// Wait for the siad process to return an error.  Ignore the error if it's a
	// SIGKILL, since we issue the process SIGKILL on quit.
	fmt.Println("> all jobs loaded.")
	err = siad.cmd.Wait()
	if err != nil && err.Error() != "signal: killed" {
		panic(err)
	}

}
